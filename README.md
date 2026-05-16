# EWS Worker — Go Microservice

Microservice Go untuk menjalankan **Early Warning System (EWS)** pada sistem **Sekolah Pintar**.

Dirancang sebagai pengganti Artisan command `ews:process` yang berjalan secara sequential di PHP. Service ini memproses seluruh siswa aktif secara paralel menggunakan worker pool goroutine, lalu menyimpan hasilnya ke `trx_ews_alerts`. Berbagi database PostgreSQL dan JWT secret yang sama dengan Laravel.

---

## Mengapa Go?

Artisan command `ews:process` yang dijadwalkan tiap pagi memproses **setiap siswa secara sequential**:

- 1.000 siswa × 3 query indicator = **3.000 query berurutan**
- PHP tidak bisa men-dispatch goroutine — setiap siswa harus selesai sebelum siswa berikutnya diproses
- Waktu eksekusi tumbuh linear seiring jumlah siswa

Go menyelesaikan ini dengan:
- **Worker pool 50 goroutine concurrent** — 1.000 siswa selesai dalam ~1/50 waktu PHP
- Connection pool 100 koneksi PostgreSQL (`SetMaxOpenConns(100)`)
- Biner tunggal, startup <100ms, konsumsi memori konstan
- Endpoint HTTP memungkinkan trigger via cron job atau scheduler eksternal

---

## Arsitektur

```
Client (Browser / Mobile)
        │
        ▼
  Nginx Reverse Proxy
    ├── /api/v1/ews*  ──→  Go EWS Worker  :8085
    └── /*            ──→  Laravel PHP-FPM :9000
              │
              ▼
         PostgreSQL (shared)
           trx_ews_alerts
           trx_absensi_siswa
           trx_nilai
           trx_bk_kasus
           mst_siswa
```

---

## EWS Logic

Untuk setiap siswa aktif, tiga indikator diperiksa secara independen:

| # | Indikator | Rule | Tabel |
|---|-----------|------|-------|
| 1 | **Absensi** | Jumlah alpha (`status=4`) ≥ 3 dalam 30 hari terakhir | `trx_absensi_siswa` |
| 2 | **Nilai** | Rata-rata nilai < 70 dalam 90 hari terakhir | `trx_nilai` |
| 3 | **Perilaku** | Ada catatan BK dalam 30 hari terakhir | `trx_bk_kasus` |

### Level Alert per Indikator

**Absensi:**

| Jumlah Alpha | Level |
|---|---|
| 3–3 | 1 — Perhatian |
| 4–4 | 2 — Waspada |
| ≥ 5 | 3 — Kritis |

**Nilai:**

| Rata-rata | Level |
|---|---|
| 60–69.99 | 1 — Perhatian |
| 50–59.99 | 2 — Waspada |
| < 50 | 3 — Kritis |

**Perilaku (BK):**

| Jumlah Kasus | Level |
|---|---|
| 1 | 1 — Perhatian |
| 2 | 2 — Waspada |
| ≥ 3 | 3 — Kritis |

Jika kondisi membaik (tidak lagi memenuhi threshold), alert aktif untuk kategori tersebut di-**auto-resolve** otomatis.

### Worker Pool

```go
sem := make(chan struct{}, 50) // maks 50 goroutine concurrent
for _, siswa := range allSiswa {
    sem <- struct{}{}
    wg.Add(1)
    go func(sw model.MstSiswa) {
        defer wg.Done()
        defer func() { <-sem }()
        checkAndUpsert(ctx, sw.ID)
    }(siswa)
}
wg.Wait()
```

---

## Struktur Proyek

```
ews-worker/
├── cmd/
│   └── main.go                     # entry point: DI, router, server
├── internal/
│   ├── config/
│   │   └── config.go               # baca env vars
│   ├── db/
│   │   └── db.go                   # koneksi PostgreSQL (sqlx + pgx), connection pool
│   ├── middleware/
│   │   └── auth.go                 # validasi JWT HS256, inject UserClaims ke context
│   ├── model/
│   │   └── models.go               # MstSiswa, TrxEWSAlert, UserClaims
│   ├── repository/
│   │   ├── errors.go               # sentinel errors
│   │   ├── ews_repo.go             # EWS queries: indicator checks, upsert, resolve, list
│   │   └── siswa_repo.go           # ListAktif, FindByID
│   ├── service/
│   │   └── ews_service.go          # batch processor + per-siswa check logic
│   └── handler/
│       ├── response.go             # helper JSON response
│       └── ews_handler.go          # HTTP handlers untuk 5 endpoint EWS
├── .env.example
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Persyaratan

| Komponen   | Versi  |
|------------|--------|
| Go         | ≥ 1.22 |
| PostgreSQL | ≥ 14   |

### Dependencies

| Package | Fungsi |
|---------|--------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/jmoiron/sqlx` | PostgreSQL query helper |
| `github.com/jackc/pgx/v5` | PostgreSQL driver (pgbouncer-safe) |
| `github.com/golang-jwt/jwt/v5` | JWT parsing & validasi |
| `github.com/joho/godotenv` | Load `.env` file (development) |

---

## Konfigurasi

```bash
cp .env.example .env
```

| Variabel | Default | Keterangan |
|----------|---------|------------|
| `APP_PORT` | `8085` | Port HTTP server |
| `DB_HOST` | `127.0.0.1` | Host PostgreSQL / pgbouncer |
| `DB_PORT` | `5432` | Port PostgreSQL |
| `DB_DATABASE` | `db_sekolah` | Nama database |
| `DB_USERNAME` | `root` | Username PostgreSQL |
| `DB_PASSWORD` | _(kosong)_ | Password PostgreSQL |
| `JWT_SECRET` | _(wajib)_ | Harus sama persis dengan `JWT_SECRET` di `.env` Laravel |

> `JWT_SECRET` wajib identik dengan Laravel agar token yang diissue Laravel dapat diverifikasi langsung oleh Go.

---

## Menjalankan

### Development

```bash
cd sekolah_go/ews-worker
cp .env.example .env   # isi JWT_SECRET dan DB credentials
go run cmd/main.go
```

### Build & Run Binary

```bash
go build -o ews-worker cmd/main.go
./ews-worker
```

### Docker

```bash
docker build -t ews-worker .
docker run -p 8085:8085 --env-file .env ews-worker
```

### Via Docker Compose (sekolah/compose.yaml)

Service sudah dikonfigurasi di `sekolah/compose.yaml`:

```bash
cd sekolah
docker compose up -d ews-worker
```

---

## Nginx Routing

Sudah dikonfigurasi di `sekolah/docker/nginx/conf.d/default.conf`:

```nginx
location ~ ^/api/v1/ews(/|$) {
    proxy_pass         http://ews-worker:8085;
    proxy_set_header   Host              $host;
    proxy_set_header   X-Real-IP         $remote_addr;
    proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header   X-Forwarded-Proto $scheme;
    proxy_set_header   Authorization     $http_authorization;
    proxy_read_timeout 60s;
}
```

---

## API Reference

Semua endpoint kecuali `/health` memerlukan header:

```
Authorization: Bearer <JWT_TOKEN>
Accept: application/json
Content-Type: application/json
```

Base URL: `http://localhost:8085`

---

### Health Check

```http
GET /health
```

```json
{ "status": "ok", "service": "ews-worker" }
```

---

### 1. Proses Batch (semua siswa aktif)

```http
POST /api/v1/ews/process
```

Memproses seluruh siswa aktif (`status = 1`) dengan worker pool 50 goroutine concurrent. Cocok dijalankan via cron job harian (pukul 06:00 WIB).

**Response**

```json
{
  "success": true,
  "message": "EWS batch processing complete",
  "data": {
    "total": 1250,
    "processed": 1248,
    "errors": 2
  }
}
```

---

### 2. Proses Satu Siswa

```http
POST /api/v1/ews/process-siswa/{siswaId}
```

Menjalankan EWS check untuk satu siswa. Berguna untuk trigger real-time setelah input absensi.

**Response**

```json
{
  "success": true,
  "message": "EWS check complete",
  "data": null
}
```

---

### 3. List EWS Alerts

```http
GET /api/v1/ews/alerts
```

| Query param | Tipe | Keterangan |
|-------------|------|------------|
| `mst_siswa_id` | integer | Filter per siswa |
| `is_resolved` | boolean (`true`/`false`) | Filter status resolved |
| `level` | integer `1–3` | Filter level alert |
| `kategori` | string | Filter: `absensi`, `nilai`, `perilaku` |
| `page` | integer | Halaman (default: 1) |
| `per_page` | integer | Per halaman (default: 20) |

**Response**

```json
{
  "success": true,
  "message": "OK",
  "data": [
    {
      "id": 42,
      "mst_siswa_id": 17,
      "kategori": "absensi",
      "level": 2,
      "pesan": "Siswa telah absent 4 hari dalam sebulan",
      "is_resolved": false,
      "resolved_at": null,
      "created_at": "2026-05-16T06:00:00Z",
      "updated_at": "2026-05-16T06:00:00Z"
    }
  ],
  "meta": { "total": 1, "page": 1, "per_page": 20 }
}
```

---

### 4. Alerts per Siswa

```http
GET /api/v1/ews/alerts/{siswaId}
```

Mengembalikan semua alert (aktif maupun resolved) untuk siswa tertentu, diurutkan dari terbaru.

**Response**

```json
{
  "success": true,
  "message": "OK",
  "data": [
    {
      "id": 42,
      "mst_siswa_id": 17,
      "kategori": "absensi",
      "level": 2,
      "pesan": "Siswa telah absent 4 hari dalam sebulan",
      "is_resolved": false,
      "resolved_at": null,
      "created_at": "2026-05-16T06:00:00Z",
      "updated_at": "2026-05-16T06:00:00Z"
    }
  ]
}
```

---

### 5. Resolve Alert

```http
PATCH /api/v1/ews/alerts/{id}/resolve
```

Menandai alert sebagai resolved secara manual (oleh guru BK atau admin).

**Response**

```json
{
  "success": true,
  "message": "alert resolved",
  "data": null
}
```

---

## Tabel Database

| Tabel | Peran |
|-------|-------|
| `mst_siswa` | Sumber daftar siswa aktif (`status = 1`) |
| `trx_absensi_siswa` | Sumber indikator absensi (`status = 4` = alpha) |
| `trx_nilai` | Sumber indikator nilai (rata-rata < 70) |
| `trx_bk_kasus` | Sumber indikator perilaku (jumlah kasus BK) |
| `trx_ews_alerts` | Target upsert alert EWS |

### Skema `trx_ews_alerts`

```sql
CREATE TABLE trx_ews_alerts (
  id             bigserial PRIMARY KEY,
  mst_siswa_id   bigint       NOT NULL,
  kategori       varchar(255) NOT NULL,   -- 'absensi' | 'nilai' | 'perilaku'
  level          int4         NOT NULL,   -- 1 | 2 | 3
  pesan          text         NOT NULL,
  data_pendukung jsonb,
  is_resolved    bool         NOT NULL DEFAULT false,
  resolved_by    bigint,
  resolved_at    timestamp(0),
  created_at     timestamp(0),
  updated_at     timestamp(0),
  deleted_at     timestamp(0)
);
```

---

## Integrasi Cron Job

Trigger batch processing setiap hari pukul 06:00 WIB via cron atau Laravel Scheduler:

**Via curl:**

```bash
0 6 * * * curl -s -X POST http://ews-worker:8085/api/v1/ews/process \
  -H "Authorization: Bearer $EWS_CRON_TOKEN" \
  -H "Content-Type: application/json"
```

**Via Laravel Scheduler (`routes/console.php`):**

```php
Schedule::call(function () {
    Http::withToken(config('services.ews.token'))
        ->post(config('services.ews.url') . '/api/v1/ews/process');
})->dailyAt('06:00')->name('ews:process')->withoutOverlapping();
```
