# Modular Monolith Architecture Blueprint

Dokumen ini mendefinisikan arsitektur target, batas domain, serta peta jalan refaktor dari arsitektur layered saat ini (`src/`) menuju **Modular Monolith** (`internal/modules/`) untuk Go backend `backend-diagram`.

---

## 1. Prinsip Utama Arsitektur

1. **Domain-First Modular Monolith, Bukan Microservices**
   - Seluruh modul berjalan dalam satu proses binary Go, berbagi siklus hidup deployment dan koneksi database pool.
   - Menghindari overhead operasional microservices (RPC serialization, distributed tracing, network latency, multiple deployment pipelines) sambil tetap menjaga batas domain yang bersih dan decoupled.

2. **Pemisahan Lapisan di Setiap Modul (Controller - Service - Repository)**
   - Setiap modul memiliki kepemilikan penuh terhadap:
     - **Controller / Transport**: HTTP request decoding, schema validation, HTTP response encoding.
     - **Service**: Aturan bisnis, koordinasi antar repository, transaksi logis.
     - **Repository**: Akses data ke PostgreSQL/Redis, query mapping ke struct domain internal.
     - **DTO & Entity**: Kontrak data internal modul.
   - Controller tidak boleh mengakses Repository atau DB langsung; semua alur data wajib melalui Service modul.

3. **Consumer-Driven Interfaces & Dependency Inversion**
   - Service mendefinisikan interface repository yang dibutuhkannya.
   - Repository mengimplementasikan interface tersebut. Hal ini membuat unit test mudah diisolasi menggunakan in-memory mock/fake tanpa ketergantungan database fisik.

4. **Isolasi Domain Diagram vs UI Design (Rule of Isolation)**
   - Domain **Diagram** (`/api/projects/*`) fokus pada flowchart, sequence, dan graph mutations.
   - Domain **UI Design** (`/api/ui-design/*`) fokus pada canvas web/mobile layout, tailwind/vue compiler, dan integrasi async sidecar CrewAI.
   - Kode, service, dan state kedua domain ini dilarang digabung atau saling bergantung secara langsung.

5. **Cross-Cutting Platform di `internal/platform/`**
   - Komponen generik yang tidak memuat logika domain (koneksi database pool, Redis wrapper, HTTP response helpers, generic middleware) diletakkan di platform cross-cutting.

---

## 2. Struktur Direktori Target

Struktur folder target yang akan dibangun secara bertahap:

```text
backend-diagram/
├── cmd/
│   └── server/
│       └── main.go                         # Composition root & dependency wiring
├── docs/
│   ├── api/
│   │   └── openapi.yaml                    # Single source of truth API contract
│   └── architecture/
│       └── modular-monolith.md             # Dokumen arsitektur ini
├── internal/
│   ├── modules/
│   │   ├── identity/                       # Modul Autentikasi & Kredensial
│   │   │   ├── controller/                 # AuthController, AdminController
│   │   │   ├── service/                    # AuthService, CredentialService
│   │   │   ├── repository/                 # PostgresCredentialRepository
│   │   │   └── dto/                        # LoginRequest, CredentialDTO
│   │   │
│   │   ├── diagram/                        # Modul Canvas Diagram & Graph
│   │   │   ├── controller/                 # ProjectController, TemplateController
│   │   │   ├── service/                    # ProjectService, RollbackService
│   │   │   ├── repository/                 # PostgresProjectRepository, VersionRepository
│   │   │   └── dto/                        # ProjectCreateRequest, GraphDTO
│   │   │
│   │   ├── uidesign/                       # Modul UI Canvas & Async Generation
│   │   │   ├── controller/                 # UIDesignController, AsyncJobController
│   │   │   ├── service/                    # UIDesignService, JobDispatcherService
│   │   │   ├── repository/                 # PostgresJobBindingRepository, UITemplateRepo
│   │   │   └── dto/                        # GenerateUIRequest, JobCallbackDTO
│   │   │
│   │   ├── workspace/                      # Modul Kolaborasi & Workspace Shared Data
│   │   │   ├── controller/                 # WorkspaceController
│   │   │   ├── service/                    # CommentService, SettingsService, ExportService
│   │   │   ├── repository/                 # CommentRepository, FoundationRepository
│   │   │   └── dto/                        # CreateCommentDTO, SettingsDTO
│   │   │
│   │   └── sharing/                        # Modul Public Share & Project Forking
│   │       ├── controller/                 # ShareController
│   │       ├── service/                    # ShareService, ForkService
│   │       ├── repository/                 # PostgresShareRepository
│   │       └── dto/                        # ShareRequestDTO, ForkDTO
│   │
│   └── platform/                           # Cross-Cutting Infrastructure
│       ├── config/                         # Environment loader & validation
│       ├── database/                       # Postgres connection, schema migrations
│       ├── redis/                          # Redis client connection
│       ├── httpresponse/                   # SendSuccess, SendError standard envelopes
│       └── middleware/                     # AuthMiddleware, RequireAdmin, InternalAuth
```

---

## 3. Batas Tanggung Jawab Modul (Bounded Contexts)

| Modul | Tanggung Jawab Utama | Tabel Database Utama |
| --- | --- | --- |
| **`identity`** | Autentikasi token bearer, verifikasi hash SHA-256, pembuatan kredensial admin. Rate limiting merupakan pekerjaan lanjutan, bukan fitur yang sudah diterapkan. | `access_credentials` |
| **`diagram`** | Pengelolaan kanvas diagram, riwayat versi (snapshot), rollback, template diagram, chat pesan diagram. | `projects`, `chat_messages`, `diagram_versions`, `diagram_templates` |
| **`uidesign`** | Generasi kanvas UI multi-frame, integrasi streaming SSE, dispatch job async ke Python sidecar, penerimaan callback worker. | `ui_templates`, `async_job_bindings` |
| **`workspace`** | Komentar kanvas, design tokens / foundations, pengaturan workspace global, log export artefak (PNG/SVG/Code). | `design_comments`, `design_foundations`, `workspace_settings`, `project_exports` |
| **`sharing`** | Pembuatan token share publik, verifikasi link read-only publik, forking project hasil share ke akun user. | `project_shares` |

`ai_usage_logs` merupakan telemetry bersama Diagram dan UI Design, bukan milik eksklusif UI Design. SQL pencatatannya berada di `internal/platform/aiusage`, tanpa aturan bisnis kanvas atau import modul domain. Kedua domain memakai recorder yang sama melalui composition root.

---

## 4. Rencana Tahapan Sprint (Roadmap)

Pekerjaan modularisasi dilakukan secara bertahap agar fungsionalitas dan test suite tidak rusak:

Sprint 2-5 berikut adalah usulan urutan ekstraksi untuk review, bukan perubahan sekaligus atau penomoran ulang sprint yang sudah disepakati. Setiap step harus lolos test dan direview sebelum berpindah modul.

- **Sprint 0: Penguatan Safety Net & Karakterisasi (Selesai)**
  - Isolasi live AI tests di balik flag `RUN_LIVE_AI_TESTS=1`.
  - Pembuatan OpenAPI contract (38 operations, 31 paths, 34 schemas).
  - Skrip linter `check_openapi.py` dan AST contract test `contract_test.go`.

- **Sprint 1: Penguatan Keamanan & Akses (Sedang Berjalan - Review Step 3)**
  - **Step 1**: Eliminasi hardcoded secret & fallback key; verifikasi startup config.
  - **Step 2**: Pengetatan kepemilikan resource (comments, exports, async jobs); proteksi callback internal.
  - **Step 3**: Penyimpanan kredensial ber-hash SHA-256, penghapusan token plaintext, migrasi transaksional idempotens.

- **Sprint 2: Ekstraksi Modul Identity & Sharing**
  - **Step 1 (Selesai - Review)**: Ekstraksi persistensi kredensial identity ke `internal/modules/identity/repository`; helper token dan kebijakan tampilan dipindahkan ke paket `identity`; `src/models/credential_model.go` menjadi facade transisi tanpa SQL.
  - **Step 2 (Selesai - Review)**: Ekstraksi aturan bisnis ke `internal/modules/identity/service`; service menerima `context.Context`, memakai port repository, dan mengembalikan tipe internal modul. Facade legacy hanya memetakan ke entity/DTO lama.
  - **Step 3 (Selesai - Review)**: Ekstraksi transport HTTP identity (controller, middleware, DTO) ke `internal/modules/identity/` dan shared envelope ke `internal/platform/httpresponse`. Request context (`r.Context()`) diteruskan langsung ke service di seluruh alur HTTP aktif. Controller dan middleware lama menjadi adapter kompatibilitas tipis untuk modul lain dan test lama.
  - **Step 4 (Selesai - Review)**: Ekstraksi domain sharing ke `internal/modules/sharing` (repository, service, controller, DTO). Akses data project dan transaksi kloning dipisahkan melalui port interface `ProjectGateway`, `ProjectAccessReader`, dan `ProjectDetailLoader`. Composition root di `main.go` dan router di `src/routes/api.go` langsung menghubungkan controller dan service sharing baru.

- **Sprint 3: Ekstraksi Modul Diagram & Workspace**
  - Pemisahan `diagram` (`ProjectService`, snapshot rollback, template model) ke `internal/modules/diagram`.
  - Pemisahan komentar, foundation, setting, dan export ke `internal/modules/workspace`.

- **Sprint 4: Isolasi UI Design & Integrasi AI Async**
  - **Step 1 (Selesai - Review)**: Ekstraksi repository template dan job binding ke `internal/modules/uidesign/repository`; SQL telemetry bersama ke `internal/platform/aiusage`. Definisi port repository pada sisi consumer dan port akses kepemilikan project, adapter kompatibilitas, serta wiring repository baru pada alur aktif.
  - **Step 2**: Ekstraksi `UIDesignService`, compiler Tailwind/Vue, serta aturan bisnis UI Design. Perluas port project untuk kebutuhan mutasi, chat, dan snapshot tanpa menyalin SQL project.
  - **Step 3**: Ekstraksi dispatcher async; rapikan kontrak request, callback, status job, dan penanganan error dengan Python sidecar.
  - **Step 4**: Ekstraksi controller dan DTO, wiring router, serta regression test kontrak HTTP/SSE.

- **Sprint 5: Konsolidasi Platform & Deprecasi `src/`**
  - Pemindahan utilities dan middleware ke `internal/platform/`.
  - Pembersihan folder `src/` legacy setelah seluruh rute berpindah ke struktur `internal/`.
  - Audit akhir race-condition dan validasi seluruh unit/characterization test.

---

## 5. Status Saat Ini

Ekstraksi Identity, Sharing, dan UI Design telah dikerjakan. **Sprint 3 Step 1 (Workspace)** telah diimplementasikan dan siap direview, dengan rincian pada bagian 9. Sprint 3 belum selesai: ekstraksi Diagram dan persistence project bersama masih merupakan pekerjaan berikutnya; akses project sementara memakai adapter legacy melalui port Workspace.

Modul identity dan modul sharing telah diekstraksi penuh ke `internal/modules/identity` dan `internal/modules/sharing`. Masing-masing modul memiliki repository, service, controller, dan DTO sendiri tanpa ketergantungan import terhadap lapisan legacy `src/` (dijaga oleh `architecture_test.go`).

Modul sharing berinteraksi dengan domain project melalui port interface murni (`ProjectGateway`, `ProjectAccessReader`, `ProjectDetailLoader`), sehingga logika tabel `project_shares` terisolasi dari tabel `projects`, `chat_messages`, dan `diagram_versions`. Kloning fork dijalankan dalam transaksi atomik dengan rollback aman pada kegagalan duplikasi node/message/version. View count increment tetap dijalankan secara asinkronus menggunakan request context terisolasi (`context.WithoutCancel`).

`src/models/share_model.go` dan `src/controllers/share_controller.go` menjadi adapter kompatibilitas tipis untuk test legacy yang masih memanggil model interface lama. Alur HTTP aktif di `main.go` dan `src/routes/api.go` langsung menggunakan `sharingcontroller.ShareController` dan `shareservice.ShareService`.

Alur HTTP identity sekarang meneruskan `r.Context()` langsung ke service dan repository, sehingga pembatalan request (client disconnect / timeout) diteruskan ke layer data. `src/controllers/auth_controller.go`, `src/controllers/admin_controller.go`, dan `src/middlewares/auth_middleware.go` dipertahankan sebagai adapter kompatibilitas untuk memfasilitasi modul legacy lain yang masih memanggil `middlewares.GetUserFromContext(ctx)`. `src/models/credential_model.go` tetap menjadi facade transisi untuk pemanggil lama berbasis `context.Background()`.

Repository sharing hanya mengakses `project_shares`. Implementasi SQL port project berada di `src/models/share_project_gateway.go` sebagai adapter transisi, bukan repository project kedua dalam modul sharing. SQL pembacaan dan kloning dipindahkan dari model share lama, bukan disalin ke modul Diagram/UI Design. Penetapan pemilik persistence bersama tetap menunggu sprint ekstraksi project.

Controller sharing meneruskan `r.Context()` ke seluruh service dan port. Service/repository sharing serta SQL gateway membaca dan mengkloning project dengan context tersebut. Namun adapter `ProjectAccessReader` dan `ProjectDetailLoader` di composition root masih memanggil `ProjectModel.GetByIDScoped` dan `ProjectService.GetProjectByID` yang belum context-aware. Dua pemanggilan legacy itu belum meneruskan pembatalan request ke database dan harus diselesaikan saat ekstraksi project; jangan menganggap seluruh jalur project sudah context-aware.

Pada boundary fork, error insert snapshot sekarang diperiksa sehingga tidak mengembalikan clone sukses tanpa versi awal; error iterasi message juga membatalkan pembacaan alih-alih mengkloning history parsial. Test SQL mock memeriksa urutan transaksi, rollback kegagalan project/message/snapshot, dan error commit. Ini perubahan penanganan error internal, bukan perubahan payload sukses atau schema database.

Token share baru tetap 16 byte acak berformat hex (32 karakter). Fallback token berbasis timestamp dihapus; kegagalan generator tidak boleh menghasilkan token yang dapat ditebak. Link lama, reaktivasi link, pemilik hasil fork, dan format URL tetap dipertahankan. Update view count tetap best-effort, menggunakan timeout 5 detik dan tidak membatalkan response ketika gagal.

Controller memakai interface service sempit yang didefinisikan pada sisi consumer, tanpa import repository/SQL. DTO HTTP hanya dimiliki modul identity; `src/dtos/auth_dto.go` mengekspor alias kompatibilitas. Helper envelope generik di `internal/platform/httpresponse` dipakai bersama melalui wrapper `src/utils/response.go`. Tidak ada import `src/` dari kode produksi modul identity (dijaga architecture test).

`src/models/credential_http_adapter.go` hanya menjembatani constructor legacy yang masih menerima model tanpa context; adapter ini tidak dipakai composition root maupun router aktif. Jangan gunakan adapter tersebut untuk fitur baru karena context request tidak dapat diteruskan melalui interface model lama. Bridge context memberi prioritas pada identitas dari middleware baru ketika context lama dan baru hadir bersamaan.

Kontrak method/path, status, envelope JSON, parsing expiry, dan default permission dipertahankan. Token mentah tetap hanya diungkap saat create dan dikembalikan sebagai `token` saat verify; field metadata `credential_key` pada profile, update, dan list tetap masked. Perilaku legacy seperti expiry input invalid yang diabaikan dan error lookup saat extend yang tetap meneruskan update belum diubah pada step pemindahan ini.

Migrasi dan bootstrap admin (termasuk validasi key saat akun belum ada) tetap menjalankan transaksi database secara atomik; validasi bootstrap berada di adapter transaksi untuk menjaga perilaku restart dan urutan SQL lama. Context startup menggunakan `context.Background()`, bukan context request. Step ini tidak menjalankan migrasi database live atau restart Docker.

Validasi lokal mencakup characterization controller, middleware public/private/admin, penerusan context yang dibatalkan, dan test router -> middleware/controller -> service -> repository dengan SQL mock. Test stack juga memeriksa kompatibilitas pembacaan user oleh modul legacy dan masking token. Ini bukan pengujian frontend atau deployment live.

Dua query lintas tabel masih dipertahankan secara eksplisit: adopsi `projects.user_id` saat bootstrap dan agregasi jumlah project untuk daftar admin. Pemindahan ini belum mengisolasi kepemilikan tabel tersebut; port/bootstrap coordinator dan read model lintas modul harus diputuskan pada step lanjutan. Jangan memecah transaksi bootstrap menjadi operasi terpisah hanya untuk mengejar pemisahan folder.

Struktur `internal/modules/` di atas masih target, bukan hasil pemindahan seluruh modul. Jangan membuat folder kosong atau memindahkan controller sebelum service memiliki batas yang jelas. Pemindahan `main.go` ke `cmd/server/` juga opsional pada tahap konsolidasi.

Tabel `projects`, `chat_messages`, dan `diagram_versions` saat ini digunakan oleh kedua mode kanvas. Pemilik persistence bersama ini harus diputuskan sebelum ekstraksi Diagram/UI Design; akses melalui port yang sempit, bukan import service atau repository internal modul lain. Modul sharing/workspace menerima port akses project yang diperlukan melalui composition root. Jangan menyalin repository project ke beberapa modul atau memindahkan semua aturan bisnis ke platform.

## 6. Sprint 4 Step 1: Persistence UI Design

- `internal/modules/uidesign/records.go` memiliki tipe template dan job binding. Alias `entities.UITemplate` dan `models.AsyncJobBinding` menjaga kompatibilitas pemanggil lama tanpa mengubah field JSON.
- `internal/modules/uidesign/service/contracts.go` mendefinisikan port `TemplateRepository` dan `JobBindingRepository`. Implementasi PostgreSQL hanya mengakses `ui_templates` dan `async_job_bindings`, menggunakan context dari consumer.
- `src/models/ui_template_model.go`, `async_job_model.go`, dan `log_model.go` tidak lagi memiliki SQL. Constructor lama tetap tersedia, tetapi alur UI Design aktif menggunakan constructor context-aware dan repository baru langsung dari `main.go`.
- `internal/platform/aiusage` memiliki recorder context-aware untuk telemetry bersama; Diagram memakai facade kompatibilitas, callback UI Design memakai recorder langsung. Waktu pencatatan, payload, dan sifat best-effort tetap dipertahankan.
- `uidesign.ProjectAccessReader` merupakan port sempit untuk pemeriksaan kepemilikan pada pembacaan status job. Adapter SQL transisi di `src/models/ui_design_project_gateway.go` mendelegasikan ke `projectModel.GetByIDScopedContext`, bukan menyalin query ke repository UI Design. Penolakan pemilik lain tetap berlaku sebelum chat dibaca.
- Pembacaan template, job binding, telemetry callback, dan pemeriksaan akses project SQL tersebut meneruskan context ke database. Mutasi project, chat, snapshot, serta fallback metadata callback masih melalui model legacy dan belum seluruhnya context-aware. Constructor kompatibilitas yang menerima model tanpa context hanya memeriksa pembatalan sebelum pemanggilan; tidak bisa membatalkan SQL yang sudah berjalan. Jangan gunakan constructor tersebut untuk wiring baru.
- Service bisnis, compiler, dispatcher, controller, DTO HTTP, dan SSE tetap di `src/` pada Step 1. Isi direktori `uidesign/service` baru berupa kontrak repository, bukan hasil ekstraksi service bisnis. Port operasi project lainnya dilanjutkan saat ekstraksi service, dengan pemilik persistence bersama dipertahankan satu tempat.
- Test SQL mock mencakup mapping template, daftar kosong `nil`, not-found, kegagalan query/scan/iterasi, validasi binding, owner/correlation, cancellation, kompatibilitas facade, dan pemakaian repository/recorder baru oleh service. Architecture test menolak import `src/` dan Diagram dari modul UI Design. Error iterasi template kini dikembalikan, bukan menghasilkan daftar sukses parsial.

Validasi Step 1 dilakukan secara lokal/offline: `make lint-spec` (38/38 operasi), `make verify`, `make test-race` (termasuk paket baru), `git diff --check`, dan `go build ./...`. Tidak ada perubahan schema/API, migrasi database live, pemanggilan AI live, restart Docker, atau deployment.

## 7. Sprint 4 Step 3: Async Job dan Integrasi Sidecar

- `internal/modules/uidesign/service/dispatcher.go` mengoordinasikan pembuatan project, binding job, antrean, dan status melalui port. `job_callback.go` menangani verifikasi callback, pemetaan frame, snapshot, chat, dan telemetry. Keduanya tidak mengimpor Redis, HTTP client, konfigurasi global, atau `src/`.
- `JobStatusStore`, `JobQueue`, dan `SidecarDispatcher` adalah port consumer di `service/contracts.go`. `ProjectGateway` yang sudah ada dipakai kembali; query project, chat, dan versi tidak disalin ke repository UI Design.
- `repository/redis_job_store.go` memusatkan key `job:%s:status`, `job:%s:project`, TTL status dua jam, dan antrean `queue:ai_orchestration`. Semua operasi memakai context; key tidak ditemukan dinormalisasi menjadi nilai kosong, sedangkan error Redis lainnya tetap diteruskan.
- `integration/http_sidecar.go` memiliki HTTP client dan kontrak transport `POST /api/v1/crews/ui-design`, termasuk header internal service key. Jika Redis tidak tersedia atau enqueue gagal, service melakukan dispatch HTTP di goroutine dengan timeout 300 detik, terlepas dari pembatalan request awal. Respons non-2xx kini dicatat sebagai error dispatch; respons async kepada pengguna tetap seperti sebelumnya.
- DTO dispatch, callback, execution metrics, dan respons async dimiliki `uidesign/dto/job_dto.go`. Alias di `src/dtos/job_dto.go` menjaga kontrak lama; `JobStatusResponse` legacy tetap membungkus tipe project legacy melalui facade.
- `src/services/job_dispatcher_service.go` hanya menjadi adapter constructor/status untuk consumer lama. Konfigurasi Redis dan HTTP diinjeksikan saat constructor, sedangkan resolusi callback URL yang sudah ada tetap dilakukan saat dispatch. Controller dan wiring akhir tetap pekerjaan Step 4.
- Ownership status diperiksa melalui durable binding dan akses project. Callback memeriksa project/correlation dari binding; fallback metadata dan mapping Redis untuk job legacy dipertahankan. Pembacaan fallback tanpa user hanya dipakai untuk callback worker setelah autentikasi internal di controller, bukan endpoint publik.
- Test offline mencakup payload antrean, fallback HTTP non-blocking, cancellation, ownership, callback tidak valid tanpa mutasi, kompatibilitas job legacy, snapshot/chat/telemetry, key/TTL Redis, dan kontrak HTTP melalui fake transport. Tidak ada request ke worker/LLM live atau deployment.

Perilaku best-effort pencatatan status, chat, snapshot, dan telemetry tetap dipertahankan. Step ini belum menambah atomic outbox, retry, callback idempotency, atau transaksi atomik lintas graph/snapshot; hal tersebut bukan jaminan dari refaktor ini. Antrean Redis dan HTTP fallback tetap menggunakan satu payload job yang sama. Tidak ada perubahan schema database atau kontrak endpoint.

## 8. Sprint 4 Step 4: Controller UI Design & Wiring Modul

- Controller UI Design dan Async Job diekstraksi ke `internal/modules/uidesign/controller/` (`ui_design_controller.go`, `async_job_controller.go`, `contracts.go`). Controller hanya berinteraksi dengan service domain, DTO, dan `internal/platform/httpresponse`. Controller dilarang mengimpor `repository`, `integration`, atau layer SQL/Redis secara langsung.
- `UIDesignController` menangani template, pembuatan desain synchronous/stream, dan iterasi chat bertarget. Header Server-Sent Events (`text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`) dan streaming progress/error diisolasi di layer transport modul.
- `AsyncJobController` menangani request dispatch async, polling status pekerjaan berdasar ownership pengguna, dan webhook callback dari sidecar AI. Validasi `X-Internal-Service-Key` dieksekusi secara konstan waktu sebelum payload didekode.
- Autentikasi dan otorisasi fitur UI (`CanGenerateUI`) diekstrak melalui port `UserReader` dan context identity (`internal/modules/identity/authcontext`), dengan fallback adapter legacy untuk pemanggil lama.
- `src/controllers/ui_design_controller.go` dan `src/controllers/async_job_controller.go` dipertahankan sebagai facade kompatibilitas (type alias dan adapter constructor) untuk menjaga interoperabilitas seluruh suite test legacy tanpa perubahan kontrak.
- Wiring server di `main.go` dan `src/routes/api.go` telah diperbarui mengarah langsung ke controller modular monolith `uidesigncontroller`, mempertahankan 100% paritas kontrak OpenAPI 3.0.3 (38 operasi pada 31 path).

## 9. Sprint 3 Step 1: Ekstraksi Modul Workspace

- Modul Workspace diekstraksi ke paket modular mandiri `internal/modules/workspace/`, mencakup subdomain kolaborasi dan preferensi:
  - **Design Comments** (`design_comments`): komentar review visual pada canvas dengan posisi koordinat node `(position_x, position_y)` dan metadata JSONB.
  - **Design Foundations** (`design_foundations`): token tema desain bawaan dan custom.
  - **Workspace Settings** (`workspace_settings`): preferensi editor global pada record `default` (mode default, tema warna, preferensi snapping canvas); belum menjadi preferensi per pengguna.
  - **Project Exports** (`project_exports`): histori artefak hasil ekspor proyek (HTML, JSON, PNG, SVG).
- Struktur layer modul Workspace:
  - `records.go`: Entitas domain murni (`DesignComment`, `DesignFoundation`, `WorkspaceSetting`, `ProjectExport`) tanpa dependensi ke layer legacy `src/`.
  - `ports.go`: Port otorisasi akses proyek `ProjectAccessReader` dan representasi `Actor`.
  - `dto/requests.go`: DTO request transport (`CreateCommentRequest`, `UpdateCommentStatusRequest`, `UpdateSettingsRequest`).
  - `repository/`: Implementasi PostgreSQL (`postgres_comment_repository.go`, `postgres_foundation_repository.go`, `postgres_settings_repository.go`, `postgres_export_repository.go`) mempertahankan query berparameter, mapping, default, dan error handling lama.
  - `service/`: Kontrak port dan logika bisnis domain (`service.go`, `comments.go`, `preferences.go`, `exports.go`, `contracts.go`) yang mengisolasi aturan otorisasi proyek, validasi konten kosong, dan default status.
  - `controller/`: Handler HTTP standar Go 1.22 (`workspace_controller.go`, `contracts.go`) terhubung langsung ke `internal/platform/httpresponse` dan context identitas `authcontext`.
- Kompatibilitas ke belakang (Backward Compatibility Facade):
  - `src/entities/project.go`: Menggunakan Go type alias (`type DesignComment = workspace.DesignComment`, dll.) sehingga seluruh pemanggil lama tetap kompatibel pada level tipe.
  - `src/models/{comment,foundation,settings,export}_model.go`: Dikonversi menjadi facade yang mendelegasikan konstruktor dan method ke repository modul baru.
  - `src/controllers/workspace_controller.go`: Mengadopsi type alias dan adaptor `WorkspaceProjectAccessModel` ke `workspace.ProjectAccessReader`.
  - `main.go` dan `src/routes/api.go`: Wiring diperbarui langsung menggunakan `workspacecontroller.WorkspaceController` modular monolith.
- Validasi offline:
  - Architecture test (`architecture_test.go`) menolak import `src/`, Diagram, dan UI Design; controller tidak mengakses repository langsung dan service tidak mengimpor HTTP/SQL. Controller tetap membaca identitas melalui `identity/authcontext`.
  - Unit test pada service, controller, repository SQL mock, dan model facade; test ownership legacy tetap dipertahankan. Coverage lokal: service 100%, repository 96.2%, controller 85%.
  - Verifikasi spec dan race detector: `make lint-spec` (38/38 operasi), `make verify`, `make test-race`, `git diff --check`, dan `go build ./...` lolos 100%.

Endpoint, bentuk JSON, status HTTP, default author `Designer`, default status `resolved`, dan pembatasan update settings ke admin tetap dipertahankan. Tidak ada penambahan fitur, migrasi database, request AI live, restart Docker, atau deployment. Pengujian memakai fake/sqlmock, bukan database produksi atau frontend live. API repository masih mengikuti signature legacy; perubahan context-aware SQL dan penghapusan facade ditunda ke tahap berikutnya agar scope ekstraksi tetap terkontrol.

## 10. Sprint 3 Step 2: Persistence Kepemilikan Core Project (Modul Diagram)

- Kepemilikan resmi tabel PostgreSQL untuk canvas graph inti diserahkan sepenuhnya ke modul Diagram di `internal/modules/diagram/repository/`:
  - `projects`: Penyimpanan node, edge, mode canvas, pin status, dan kepemilikan user.
  - `diagram_versions`: Riwayat snapshot canvas rollback dan perubahannya.
  - `chat_messages`: Percakapan prompt dan instruksi iterasi node diagram.
  - `diagram_templates`: Template bawaan diagram alur/arsitektur.
- Struktur repository & entitas domain:
  - `internal/modules/diagram/records.go`: Entitas domain `Project`, `ProjectSummary`, `ChatMessage`, `DiagramVersion`, `DiagramTemplate`, dan `CloneInput`.
  - `repository/postgres_project_repository.go`: Mengimplementasikan `ProjectRepository` (CRUD, scoped query, paginasi, pin toggle, update graph).
  - `repository/postgres_version_repository.go`: Mengimplementasikan `VersionRepository` (versi snapshot, perolehan versi terbaru, detail versi).
  - `repository/postgres_message_repository.go`: Mengimplementasikan `MessageRepository` (append chat message dan pengambilan riwayat berurutan).
  - `repository/postgres_template_repository.go`: Mengimplementasikan `TemplateRepository` (pencarian template berdasarkan popularitas/kategori).
  - `repository/postgres_clone_repository.go`: Penanganan transaksi kloning project dan chat messages secara atomik untuk fitur fork/share.
- Facade Kompatibilitas:
  - `src/entities/project.go`: Menggunakan type alias (`type Project = diagram.Project`, dll.) sehingga kode dependensi consumer lama tetap valid.
  - `src/models/{project,version,message,template}_model.go`: Menjadi adapter delegasi tipis ke konstruktor repository modul baru.
  - `src/models/share_project_gateway.go`: Memanfaatkan `PostgresCloneRepository` dari modul Diagram untuk alur forking yang konsisten.
  - `main.go`: Diperbarui untuk menginjeksi repository modul diagram secara langsung.
- Pengujian & Validasi Arsitektur:
  - `architecture_test.go`: Menolak import `src/`, `uidesign`, `workspace`, dan `sharing` langsung dari dalam modul diagram.
  - Unit test sqlmock mencakup transaksi kloning, query berhalaman, akses scoped antar pemilik, pembaruan graph, dan model facade.
  - Verifikasi: `make lint-spec`, `make verify`, `make test-race`, `git diff --check`, `go build ./...` lolos 100%.

## 11. Sprint 3 Step 3: Service Domain Diagram

- `internal/modules/diagram/dto/project_dto.go` memiliki kontrak project, graph, chat, targeted selection, pagination, version, dan template. `src/dtos/project_dto.go` menggunakan type alias agar consumer Diagram maupun UI Design tetap kompatibel. DTO transport provider OpenAI tetap berada di adapter legacy, bukan domain Diagram.
- `internal/modules/diagram/service/contracts.go` mendefinisikan port repository sesuai kebutuhan consumer dan `AIClient`. Service menerima recorder bersama `aiusage.Recorder`; tidak mengimpor model legacy atau implementasi repository PostgreSQL.
- `service/projects.go` menangani pembuatan template/blank/AI, pembacaan scoped, pembaruan graph, pagination, dan pin. `chat.go` menangani history, graph terkini, targeted node, deteksi usecase, pesan assistant, serta snapshot. `versions.go` menangani riwayat dan rollback; `templates.go` menangani daftar template.
- `src/services/project_service.go` menjadi facade tipis melalui embedding service modular. Adapter di facade menghubungkan engine AI lama dan telemetry legacy. Constructor dan method publik lama tetap tersedia; controller dan wiring router akhir belum dipindahkan pada step ini.
- Prompt, parsing output, dan HTTP provider masih dimiliki `src/services/ai_service.go`, diakses hanya lewat port. Ekstraksi ini tidak mengklaim seluruh transport AI sudah modular atau context-aware: signature repository/AI masih mengikuti perilaku legacy. Telemetry tetap best-effort di goroutine dengan background context.
- Test offline memakai fake AI dan repository in-memory, mencakup creation, ownership, history/current graph, targeted IDs, snapshot trigger, rollback lintas project, pagination, pin, template, serta error dependency. Architecture test menolak import legacy/lintas modul, akses repository/SQL/HTTP/config/Redis dari service, dan ketergantungan DTO ke layer implementasi.

Validasi lokal Step 3: `make lint-spec` (38/38 operasi cocok), `make verify`, `make test-race`, `git diff --check`, dan `go build ./...` lolos. Coverage statement service Diagram pada test paketnya: 92.8%.

Tidak ada perubahan endpoint, bentuk JSON, schema database, atau fitur. Perilaku best-effort pada chat/snapshot template dan blank, urutan write sebelum AI, serta ketiadaan transaksi atomik lintas graph/chat/version tetap mengikuti implementasi sebelumnya. Pengujian tidak memakai AI live, database produksi, atau frontend live; tidak ada restart Docker maupun deployment.

## 12. Sprint 3 Step 4: Ekstraksi Controller Diagram & Wiring Router Akhir

- Kepemilikan HTTP handler untuk Diagram dipindahkan sepenuhnya ke `internal/modules/diagram/controller/`:
  - `contracts.go`: Port consumer `DiagramService`, model identitas `AuthenticatedUser`, dan fungsi injeksi pembaca identitas `UserReader` berbasis `authcontext.UserFromContext`.
  - `project_controller.go`: 9 HTTP handler native Go 1.22 (`Create`, `Chat`, `GetByID`, `UpdateGraph`, `GetAll`, `TogglePin`, `GetVersions`, `Rollback`, `GetTemplates`). Parameter path, query pagination, status HTTP, pesan, dan envelope JSON dipertahankan. Guard `CanGenerateDiagram` tetap pada Create/Chat sebagaimana sebelumnya; pembatasan akses project tetap diteruskan ke service. Respons memakai `internal/platform/httpresponse`.
- Adapter & Kompatibilitas Facade:
  - `src/controllers/project_controller.go`: Menjadi type alias `type ProjectController = diagramcontroller.ProjectController`. Constructor `NewProjectController` menyediakan fallback pembaca context ganda (`authcontext` dan `middlewares.GetUserFromContext`) agar pengujian legacy tetap kompatibel.
  - `src/services/diagram_ai_adapter.go`: Menyediakan adapter `NewDiagramAIClient` yang membungkus `AIService` warisan ke dalam interface `diagramservice.AIClient`.
  - `src/routes/api.go`: Menerima `*diagramcontroller.ProjectController` secara eksplisit, selaras dengan modul modular monolith lainnya.
  - `main.go`: Wiring langsung menggunakan `diagramservice.NewDiagramService` dan `diagramcontroller.NewProjectController`, termasuk injeksi recorder telemetry bersama tanpa facade model log legacy. Transport/prompt AI masih berada di legacy melalui adapter; belum seluruh kode `src/` dihapus.
- Validasi Arsitektur & Pengujian Offline:
  - `internal/modules/diagram/architecture_test.go`: Menolak import legacy, Sharing/Workspace/UI Design, repository, `database/sql`, konfigurasi lingkungan, dan Redis dari controller. Dependensi identitas melalui `identity/authcontext` tetap diperbolehkan.
  - Unit test HTTP handler di `internal/modules/diagram/controller/controller_test.go` mencakup invalid body (400), guard izin `CanGenerateDiagram` (403), missing parameter path, query pagination, kegagalan service (500/404), dan respons sukses (201/200), dengan statement coverage 93.6%.
  - Unit test facade di `src/controllers/project_controller_test.go` memastikan kompatibilitas caller lama.
  - `src/routes/diagram_test.go` menguji kesembilan rute melalui ServeMux dan middleware identitas nyata dengan service/finder fake: forwarding user/role, ID path, target node, payload graph, pagination, envelope JSON, 401 tanpa autentikasi, serta 403 untuk generate/chat tanpa izin. Test ini offline, bukan pengujian database/LLM/ frontend live.
  - Verifikasi: `make lint-spec` (38/38 operasi cocok), `make verify`, `make test-race`, `go test -race ./src/routes`, `git diff --check`, dan `go build ./...` lolos.

Dengan selesainya Step 4, seluruh modul inti Diagram (`internal/modules/diagram/`) telah lengkap (records, repository, service, DTO, dan controller). Tidak ada migrasi database, perubahan endpoint API publik, restart Docker, atau pemanggilan AI live.

## 13. Sprint 5 Step 1: Konsolidasi Platform Middleware Cross-Cutting

- Pemisahan komponen middleware HTTP generik lintas modul ke `internal/platform/middleware/`:
  - `cors.go`: `CORSMiddleware(next http.Handler, allowedOriginsHeader string) http.Handler` menangani preflight `OPTIONS`, matching origin wildcard/localhost/127.0.0.1, serta header keamanan standar browser.
  - `logger.go`: `LoggerMiddleware(next http.Handler) http.Handler` mencatat method, URL path, HTTP status code, dan durasi request dengan `responseRecorder` yang mendukung `http.Flusher`.
  - `internal_auth.go`: `ValidInternalKey(expectedKey string, r *http.Request) bool` dan `RequireInternalKey(expectedKey string, next http.HandlerFunc) http.HandlerFunc` melakukan autentikasi timing-safe SHA-256 untuk worker callback Python tanpa memuat dependensi domain kanvas atau config global.
- Kompatibilitas Facade di `src/middlewares/`:
  - `src/middlewares/cors.go`, `logger.go`, dan `internal_auth.go` mendelegasikan eksekusi ke paket `internal/platform/middleware/`, membaca konfigurasi fallback saat runtime sehingga pengujian lama tetap berjalan tanpa perubahan konfigurasi.
  - `main.go`: Diperbarui untuk menggunakan `platformmiddleware.LoggerMiddleware`, `platformmiddleware.CORSMiddleware`, dan fungsi validator internal service key langsung.
- Pengujian & Verifikasi:
  - Unit test mandiri di `internal/platform/middleware/middleware_test.go` mencapai statement coverage 95.7%.
  - Target `Makefile` diperbarui mengikutsertakan `./internal/platform/...` dan paket rute ke dalam `test-race`.
  - Seluruh verifikasi offline (`make lint-spec`, `make verify`, `make test-race`, `git diff --check`, `go build ./...`) lolos 100%.

## 14. Sprint 5 Step 2: Konfigurasi & Koneksi Infrastruktur

- `internal/platform/config/` memiliki `EnvConfig`, `LoadEnv() *EnvConfig`, dan `Validate() error`. Nama environment variable, fallback, pemilihan provider, serta validasi kredensial tetap mengikuti perilaku sebelumnya. Paket ini tidak menyimpan singleton konfigurasi global.
- `internal/platform/database/` memiliki `Config`, `Connect(ctx, Config) (*sql.DB, error)`, `RunMigrations(ctx, *sql.DB) error`, dan `Close(*sql.DB)`. Koneksi memakai `PingContext`, konfigurasi pool tetap dipertahankan, dan koneksi ditutup bila ping gagal. Script `init.sql` di-embed dari paket ini; duplikat di `src/config/` dihapus. Mount Docker Compose dan referensi SECURITY.md mengikuti lokasi baru.
- `internal/platform/redis/` memiliki `Connect(ctx, redisURL) (*redis.Client, error)` dan `Close(*redis.Client)`. Ping dibatasi dua detik dengan context turunan; client ditutup ketika ping gagal. Keputusan melanjutkan aplikasi tanpa Redis tetap berada di wiring aplikasi, bukan di paket koneksi.
- `main.go` memuat konfigurasi platform, membuka koneksi PostgreSQL/Redis, dan menginjeksi referensinya ke dependency aplikasi secara eksplisit. Signature koneksi menerima context, tetapi startup saat ini masih memakai `context.Background()`; step ini belum menambahkan timeout startup PostgreSQL atau propagasi context ke seluruh repository. Migrasi otomatis saat startup tetap dipertahankan, termasuk perilaku warning bila gagal.
- `src/config/{env,database,redis}.go` tetap menjadi facade kompatibilitas. Global `Env`, `DB`, dan `RedisClient` tersedia untuk caller lama; `main.go` masih mengisi `config.Env` untuk adapter AI legacy. Penghapusan seluruh dependensi legacy belum termasuk step ini.
- Unit test mencakup konfigurasi/validasi environment, koneksi dan eksekusi migrasi PostgreSQL dengan sqlmock, error koneksi, serta pembatalan context Redis tanpa server live. Architecture test pada ketiga paket menolak import `src/` dan modul domain.

Validasi offline: `make lint-spec` (38/38 operasi cocok), `make verify` (go vet dan seluruh unit test), `make test-race`, `git diff --check`, dan `go build ./...`. Tidak ada perubahan kontrak API atau isi schema SQL pada step ini; tidak ada migrasi database live, restart Docker, deployment, maupun pemanggilan AI live selama verifikasi.

## 15. Sprint 5 Step 3: Adapter Integrasi, LLM Platform & Wiring Aplikasi

- `internal/platform/llm/` mengonsolidasi engine transport HTTP untuk provider AI (OpenAI & Anthropic), parsing stream SSE, exponential backoff with jitter retry, serta sanitasi respons JSON secara mandiri tanpa dependensi domain modul maupun `src/`.
- `internal/modules/diagram/integration/` memuat `AIClient` dan konstruksi system prompt diagram, mengisolasi domain diagram dari protokol transport HTTP mentah.
- `internal/modules/uidesign/integration/ai_client.go` menyediakan adapter `uidesignservice.LLMClient` berbasis engine transport bersama.
- `internal/integration/projectgateway/` bertindak sebagai batas integrasi netral antar modul tanpa ketergantungan import terhadap `src/`:
  - `SharingProjectGateway`: Menghubungkan domain Sharing ke `diagramrepository.PostgresCloneRepository` untuk kloning fork dan pembacaan project bersama.
  - `UIDesignProjectGateway`: Menghubungkan domain UI Design ke repository Project, Message, dan Version Diagram dengan propagasi context pembatalan request.
  - `WorkspaceProjectAccessReader`: Memvalidasi kepemilikan project untuk komentar, foundation, setting, dan export Diagram secara context-aware.
- `internal/app/` (`app.go`, `routes.go`) menjadi composition root resmi aplikasi. `NewHandler` merangkai dependency injection seluruh modul internal, gateway platform, middleware, dan ServeMux native Go 1.22 tanpa mengimpor paket `src/`.
- `main.go` kini hanya bertanggung jawab atas bootstrap lifecycle server (environment parsing, inisialisasi koneksi database/Redis, graceful shutdown) dan mendelegasikan pembuatan HTTP handler ke `app.NewHandler`.
- Kompatibilitas Facade di `src/models/` dan `src/services/` dipertahankan sebagai wrapper tipis untuk test legacy yang belum dimigrasi.
- Target `Makefile` diperbarui mengikutsertakan `./internal/integration/...` dan `./internal/app/...` dalam `test-race`.

Validasi offline: `make lint-spec` (38/38 operasi cocok), `make verify` (seluruh go vet dan unit test lolos), `make test-race` (lolos tanpa race condition), `git diff --check`, dan `go build ./...`. Tidak ada migrasi database live, perubahan kontrak API, restart Docker, deployment, atau pemanggilan AI live selama pengerjaan.

## 16. Sprint 5 Step 4: Pembersihan Facade Legacy & Audit Akhir

- Deprekasi Formal Layer Legacy `src/`:
  - Seluruh entry point facade (`src/config/`, `src/controllers/`, `src/dtos/`, `src/entities/`, `src/middlewares/`, `src/models/`, `src/routes/`, `src/services/`, `src/utils/`) telah ditandai dengan dokumentasi godoc `// Deprecated: ...`.
  - Seluruh alur produksi aktif di `main.go` tidak lagi memiliki dependensi terhadap `src/` (0 import).
  - Rute aktif, registrasi ServeMux Go 1.22, dan error handler seluruhnya berada di bawah `internal/app/` dan modul domain masing-masing.
- Audit Batas Arsitektur (Isolation Guards):
  - Seluruh modul domain (`identity`, `sharing`, `diagram`, `uidesign`, `workspace`) memiliki `architecture_test.go` otomatis yang memverifikasi isolasi modular dan menolak import terhadap `src/`.
  - Seluruh paket platform (`config`, `database`, `redis`, `httpresponse`, `middleware`, `llm`) dan integration (`projectgateway`, `app`) memiliki `architecture_test.go` yang memastikan tidak ada import balik ke `src/` maupun coupling ke domain.
- Audit Test Suite & Kepatuhan Kontrak:
  - `make lint-spec`: Paritas 100% dengan OpenAPI 3.0.3 (38 operasi, 31 path, 34 schema).
  - `make verify`: `go vet ./...` bersih dan seluruh unit test di `internal/` maupun characterization test di `src/` lolos tanpa kegagalan.
  - `make test-race`: Lolos pada seluruh paket tanpa data race.
  - `git diff --check`: Bersih tanpa trailing whitespace atau konflik merge.
  - `go build ./...`: Kompilasi sukses 100%.

Sprint 5 selesai secara penuh. Seluruh arsitektur kini telah bertransformasi menjadi Modular Monolith yang terstruktur, maintainable, dan terisolasi dengan rapi. Refactoring dilakukan secara offline tanpa perubahan kontrak API, tanpa migrasi database live, dan tanpa deployment container live.
