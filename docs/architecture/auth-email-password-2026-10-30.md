# Email/Password Authentication

Tanggal: 30 Oktober 2026.
Status: implementasi, pengujian, rebuild, migration, dan provisioning pada environment lokal selesai. Bukan deployment VPS. Google OAuth dan signup publik ditunda sesuai permintaan.

## Kontrak Dan Batas

- `POST /api/auth/login`: JSON email/password, respons envelope `data.user` dan `data.csrf_token`, serta session cookie HttpOnly. Tidak mengembalikan bearer token/JWT/session secret dalam JSON.
- `GET /api/auth/session`: restore profil dan CSRF token berdasarkan cookie aktif. Semua respons auth memakai `Cache-Control: no-store`.
- `DELETE /api/auth/session`: CSRF-protected, revoke sesi di database dan expire cookie. Alias POST logout lama juga mencabut sesi browser; kegagalan storage tidak diklaim sebagai logout sukses.
- Session 32 byte acak, umur absolut 12 jam, hanya SHA-256 token tersimpan. Login mengganti sesi cookie sebelumnya tanpa memutus sesi perangkat lain. Akun dinonaktifkan/kedaluwarsa diperiksa pada setiap request.
- Cookie `vw_session`, Path `/`, HttpOnly, SameSite Lax. `AUTH_COOKIE_SECURE` default true; Compose lokal mengizinkan false untuk HTTP. Compose production menetapkan true. Tidak mempercayai X-Forwarded-* untuk menentukan keamanan cookie/IP.
- Password Argon2id dengan salt acak 16 byte, memory 19456 KiB, time 2, parallelism 1, key 32 byte. Provisioning membutuhkan 15..256 byte; login tidak melakukan trim password. Hash decoder membatasi parameter agar database value tidak memicu alokasi tak terbatas.
- CSRF token HMAC berbasis session secret diberikan melalui respons JSON dan dipakai pada header `X-CSRF-Token` untuk request tidak aman. Origin yang dikirim wajib cocok dengan allowlist; cross-site Fetch Metadata ditolak. Login hanya menerima application/json, tanpa content-type sederhana, dan body maksimal 4096 byte dengan strict decoder.
- Email dinormalisasi lowercase/trim; email tak dikenal, password salah, akun nonaktif/expired memakai error generik. Dummy verifier dipakai untuk email tak dikenal. Percobaan dibatasi di PostgreSQL: 10 per email/15 menit, 100 per direct peer IP/15 menit, serta maksimal 4 verifier password bersamaan per proses.
- CORS menggunakan allowlist eksplisit, bukan wildcard atau pengecualian localhost otomatis. Header CSRF diizinkan pada preflight; Vary Origin dipertahankan.

## Transisi Identity Lama

Migration baru `08_password_sessions.sql` menambah `password_accounts`, `browser_sessions`, dan `password_login_attempts`. Account terhubung ke ID `access_credentials` yang sudah dipakai sebagai principal/owner/permission, bukan membuat ID user paralel yang kehilangan project.

Login bearer key dan `/api/auth/verify` dinonaktifkan secara default (`AUTH_LEGACY_CREDENTIALS=false`; verify mengembalikan 404). Opt-in true tersedia hanya untuk transisi consumer lama; logout bearer lama bersifat stateless. Data credential tidak dihapus. Bootstrap identity pertama pada mode password tidak lagi mewajibkan pengguna membuat master credential key; key internal acak yang tidak ditampilkan dapat dipakai untuk mengisi struktur identity lama.

Vue tidak diubah dan tidak disambungkan ke session baru. Jika consumer lama masih memerlukan bearer, opt-in hanya setelah memahami risikonya. React menggunakan cookie session.

## Perbaikan Startup Legacy

Backend lokal sebelumnya restart-loop karena enam pesan legacy memiliki `target_node_ids` bertipe object, sedangkan schema connected workspace mewajibkan array. SQL NULL/JSON null juga perlu dinormalisasi.

Importer backfill yang dijalankan ulang, `04_connected_workspace_backfill.sql`, kini menerima array apa adanya, mengubah target non-array menjadi array kosong, dan menyimpan nilai non-array non-null ke metadata `legacy_target_node_ids`. Tabel sumber lama tidak diubah. Tidak mengubah checksum migration schema 04..07 yang sudah tercatat. Perilaku baru diuji pada fixture null/object dan salinan backup database lokal.

## Operasional Lokal Yang Sudah Dilakukan

1. Menjalankan kembali PostgreSQL/Redis dengan volume `backend-diagram_pgdata` yang ada; tidak memakai down -v/reset/drop.
2. Backup format custom dibuat di `/home/bobby/.local/share/visual-workspace/backups/before-password-auth-20261030.dump`, permission 0600 pada direktori privat.
3. Restore backup ke PostgreSQL disposable, menjalankan migration dan repeat, memastikan jumlah 88 project tetap. Ini dilakukan sebelum menerapkan migration pada database aplikasi.
4. `docker compose -f docker-compose.yml up -d --build backend`; startup berhasil dan health 8080 healthy, ledger migration 04..08 tersedia. PostgreSQL/Redis aktif; service Python tidak diperlukan untuk login dan tidak dijalankan ulang pada tahap ini.
5. Membuat akun development `developer@visualworkspace.test`, ditautkan secara eksplisit ke identity administrator lokal lama supaya project lama tetap terlihat. Password acak disimpan hanya di `/home/bobby/.local/share/visual-workspace/dev-login.txt`, permission 0600, di luar repository. Tidak menaruh password dalam dokumen, log, environment frontend, image, atau commit.
6. Menjalankan React dev server port 5174 dan memverifikasi login/session/logout dengan backend/database lokal nyata. Tidak memanggil provider LLM asli atau membuat project QA di database pribadi.

Provisioning menggunakan `go run ./cmd/create-account` dengan `-email`, `-name`, `-password-file` dan opsional `-existing-user-id`. Berikan konfigurasi DB environment yang benar; jangan mengandalkan default localhost:5432 untuk PostgreSQL Compose yang dipublikasikan di 5433. Akun baru biasa memperoleh role user; opsi existing-user-id mempertahankan role/permission/ownership yang sudah ada. Provisioning menolak overwrite akun/email dan tidak menawarkan reset password terselubung.

## File Implementasi

- `internal/modules/identity/password.go`, `password_test.go`: normalisasi email dan password policy/hash.
- `internal/modules/identity/repository/browser_repository.go`: account, rate bucket, session hash persistence, expiry/revoke.
- `internal/modules/identity/repository/provision_account.go`: provisioning transaksional atau penautan principal lama.
- `internal/modules/identity/service/browser_service.go`: login, rate/verifier guard, sesi, CSRF, permission/expiry checks.
- `internal/modules/identity/middleware/browser.go`: autentikasi session untuk route aplikasi, CSRF/origin, legacy gate.
- `internal/modules/identity/controller/browser_controller.go`, `auth_controller.go`: strict HTTP login, profile/session, cookie/logout.
- `internal/app/app.go`, `routes.go`, `main.go`: wiring dan bootstrap.
- `internal/platform/config/env.go`, `middleware/cors.go`, `.env.example`, Compose lokal/production: konfigurasi cookie/legacy dan CORS.
- `internal/platform/database/migrations.go`, `migrations/08_password_sessions.sql`, `migrations/04_connected_workspace_backfill.sql`: schema baru dan importer legacy.
- `internal/platform/database/password_auth_integration_test.go`, `postgres_test.go`, `connected_workspace_migration_test.go`, `diagram_generation_integration_test.go`, `internal/platform/middleware/stack_test.go`, `middleware_test.go`: regression dan security coverage.
- `cmd/create-account/main.go`: provisioning account dengan password file privat exclusive-create.
- `go.mod`, `go.sum`: Argon2 melalui golang.org/x/crypto v0.55.0; Go tetap 1.25, tidak memigrasikan toolchain project.
- `docs/api/openapi.yaml`, `scripts/check_openapi.py`: kontrak 46 paths, 59 operasi, 78 schema.

File frontend dan alur UI: `../../../frontend-react/docs/auth-email-password-2026-10-30.md`.

## Verifikasi

- `make verify`, `make lint-spec`: PASS.
- Race tests identity, app, config, middleware: PASS.
- PostgreSQL disposable dengan race: password auth, legacy null/object backfill, connected workspace, upgrade diagram, targeted mutation, dan generation runs PASS.
- Auth integration menguji hashing/non-plaintext, email normalisasi, exact whitespace password, error generik, invalid/oversized JSON, media type sederhana ditolak, bearer dinonaktifkan, CSRF/origin, cookie flags, token rotation, dua logout endpoint dan replay, expiry/deactivation, ownership saat write, rate limit, duplicate provisioning rollback, dan repeat migration.
- Backup lokal di-restore dan migration/repeat diuji di database disposable; 88 project tetap ada. Database aplikasi setelah startup juga tetap 88 project.
- React lint/build serta Chrome desktop/mobile dengan backend nyata PASS, termasuk storage/token privacy, pending prompt, reload, CSRF, gagal logout/retry dan revocation. Bukti sementara `/tmp/password-auth-{verify,race,integration,snapshot,openapi}.log`, `/tmp/password-auth-check/`, dan `/tmp/password-auth-docker-build.log`.

## Belum Termasuk

Google OAuth, signup/verifikasi email, reset/change password, manajemen/revoke semua sesi perangkat, MFA, idle timeout, audit keamanan/performa penuh, dan deployment VPS. Lifetime sesi saat ini absolut 12 jam. Di belakang reverse proxy, limiter memakai peer langsung sehingga pengguna dapat berbagi IP bucket; konfigurasi trusted proxy/rate-limit edge perlu dirancang sebelum publikasi skala lebih besar. Gunakan HTTPS, origin eksplisit, dan secret management saat deployment; jangan menyalin password development ke production.
