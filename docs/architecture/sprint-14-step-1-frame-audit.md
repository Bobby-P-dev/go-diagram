# Sprint 14 Step 1: Audit Wireframe Dan UI Design

Tanggal: 30 Oktober 2026
Status: audit dan kontrak target selesai; implementasi runtime belum diaktifkan.

## Tujuan Dan Batas

Memindahkan generation wireframe/UI ke workspace project-thread-artifact yang
sudah dipakai Brainstorm, PRD, dan diagram. Bukan redesign homepage, bukan
membuat microservice, dan bukan mengganti autentikasi.

Step ini menghasilkan audit source, keputusan batas modul, kontrak frame v1,
dan backlog implementasi bertahap. Tidak mengubah Go/React, API aktif, SQL,
environment, Docker, atau data pengguna. Tidak memanggil provider berbayar.

Eksplorasi memakai fallback pembacaan source terarah karena tool graph
codebase-memory tidak tersedia pada sesi ini. Audit dibatasi pada jalur yang
tercantum di bawah; bukan audit keamanan menyeluruh atau sertifikasi provider.
Instruksi skill yang masih menyebut `src/`/Vue tidak dijadikan deskripsi keadaan
runtime baru: backend sekarang memakai `internal/`, frontend target React.

## Hasil Audit

| Area | Keadaan aktual | Implikasi Sprint 14 |
| --- | --- | --- |
| Artifact | `artifact/validation.go` memetakan `wireframe`/`ui_design` ke `frame`; validasi frame baru mewajibkan object dengan arrays `nodes`/`edges` | Storage dasarnya tersedia, tetapi belum memvalidasi semantik/safety frame baru |
| Generation | `generation/candidate.go:OutputFormat` hanya menerima Brainstorm/PRD/ERD/flowchart/sequence | Mengganti pilihan frontend saja tidak cukup; wireframe/UI masih ditolak |
| Coordinator | `generation/service/run_generation.go` hanya dispatch ke document/diagram engine | Perlu port frame tersendiri dan cabang dispatch eksplisit, bukan memperlakukan frame sebagai Markdown |
| Persistence | `generation/repository/postgres_commit.go` hanya menyusun document atau graph; INSERT revision tidak menulis `frame_data` | Frame harus ikut commit atomik dengan revision, message, head, context links dan status run |
| Database run | Migration 06 membatasi output type dan base snapshot document/graph; migration 07 membatasi mutation ke diagram | Perlu migration baru untuk frame types dan `base_frame`; jangan mengedit migration lama |
| Legacy compiler | `uidesign/service/ui_design_compiler.go` memiliki synthesis, requirement/design reconstruction, validator, frame assembly | Konsep/komponen murni dapat digunakan kembali setelah diisolasi; jangan panggil service penyimpan legacy dari coordinator baru |
| Legacy output | `uidesign/dto/ui_design_dto.go:UIFrameData` menyimpan canonical state sekaligus alias `raw_html`, `sections`, `code_export` | Perlu adapter eksplisit, bukan dua payload canonical yang dapat berbeda |
| Repair | `uidesign/service/ui_synthesis.go` punya satu generation dan maksimal satu repair, disertai sanitasi HTML dan validasi | Pertahankan repair terbatas; hasil invalid tidak boleh diganti template lalu dianggap sukses |
| Legacy jobs | `uidesign/service/dispatcher.go` memiliki queue/sidecar/status/callback dan project gateway sendiri | Tidak membuat lifecycle job kedua untuk generation v2; jalur lama tetap terpisah selama transisi |
| Context | `generation/context.go`, `service/context_resolver.go`, dan frozen manifest pada run sudah tersedia | Pakai snapshot source project yang sama, bukan membaca seluruh chat atau project lain dari prompt |
| React | `ArtifactList.tsx` bisa membuat frame kosong; `ArtifactStage.tsx` masih textarea JSON; `GenerationPanel.tsx` belum menyediakan generation frame | Storage JSON bukan renderer atau generator wireframe/UI yang sudah siap |
| Legacy preview | Vue `UiFrameNode.vue` sudah memiliki preview/sandbox | Referensi perilaku saja; tidak menyalin implementasi Vue atau membawa dependency/runtime Vue ke React |

Belum ditemukan engine wireframe terpisah pada jalur generation v2 yang diaudit.
Wireframe harus menjadi output type eksplisit dengan fidelity rendah, bukan UI
berwarna yang hanya diganti namanya. Jalur Python/sidecar tidak diubah atau
dinyatakan setara kontraknya pada step ini.

## Temuan Penting

1. Menambahkan `wireframe` ke enum saja akan menyisakan gap dispatch, snapshot,
   validasi, dan INSERT revision. Aktivasi wajib satu slice yang konsisten.
2. Sanitizer legacy menolak sejumlah active tags dan event handlers, tetapi
   `safeStaticURL` masih mengizinkan HTTP(S), mailto/tel, dan URL relatif.
   Ini bukan jaminan preview tanpa jaringan. CSS, URL, sandbox dan CSP wajib
   menjadi boundary sendiri sebelum HTML dirender pada aplikasi yang login.
3. `UIDesignCompiler` memakai LLM interface legacy tanpa parameter context pada
   method panggilan. Pengecekan ctx sebelum/sesudah call tidak membuktikan call
   provider bisa dihentikan di tengah jalan. Engine baru memakai port LLM
   context-aware dari generation, dengan timeout/cancellation yang diuji.
4. Validasi artifact `frame` saat ini terlalu umum untuk menjamin frame v1 aman.
   Validasi canonical perlu diterapkan pada hasil AI, manual save, dan restore
   yang mengklaim kontrak baru. Artifact legacy tetap punya jalur kompatibilitas
   read-only/fallback, bukan migrasi diam-diam ketika dibuka.
5. Tidak ada alasan membuat microservice baru: gap utamanya kontrak domain dan
   integrasi lifecycle, bukan pemisahan deployment.

## Keputusan Arsitektur Target

- `generation`: koordinasi run, context, candidate, cancellation, idempotency,
  dan transactional persistence; tidak mengandung template visual bisnis.
- `uidesign`: aturan frame, validator, sanitizer, prompt frame, dan engine murni
  tanpa menulis project, revision, message, atau job status sendiri.
- `artifact`: pemilik snapshot/revision serta validasi payload yang disimpan;
  memakai validator frame melalui batas integrasi yang jelas bila diperlukan,
  bukan mengimpor concrete service/repository uidesign.
- `internal/integration`: adapter compiler/LLM atau legacy DTO bila dibutuhkan;
  app composition root menyuntikkan dependency. Hindari import cycle.
- React: fitur frame renderer/editor terpisah dari diagram, dimuat lazy;
  komponen shell/toolbar boleh reusable, isi hasil AI bukan preset dashboard.
- Kontrak target ada di `../contracts/frame-v1.md`. Ini keputusan implementasi
  Step 2, bukan klaim endpoint sudah menerima payload tersebut sekarang.

## Urutan Sprint 14

### Step 1 - Audit Dan Kontrak (Selesai)

- Peta source/lifecycle aktual, gap keamanan/kompatibilitas, keputusan canonical.
- Kontrak output, input options, context, revision, resource limits, dan review
  checklist disiapkan tanpa mengaktifkan generation baru.

### Step 2 - Backend Frame Generation Dan Persistence

1. Tambah typed FrameInput/FrameCandidate dan FrameEngine port; implementasikan
   engine tanpa persistence di uidesign/integration dengan context-aware LLM.
2. Implementasikan parser strict, limits, safety validation dan satu repair;
   wireframe/UI berbagi struktur, bukan berbagi template bisnis.
3. Tambah migration baru setelah nomor migration tertinggi aktual: perluas
   output types, simpan `base_frame`/options, perketat snapshot constraints.
4. Perluas coordinator, candidate validation, run scanning, hashing idempotency,
   target snapshot dan commit `frame_data` secara atomik. Query SQL harus
   menulis versi kontrak dari candidate yang sudah diverifikasi.
5. Perbarui OpenAPI bersamaan aktivasi, bukan sebelum runtime tersedia.
6. Uji mock engine/provider, cross-project denial, repair failure, cancellation,
   retry, rollback, stale target, source links, serta migration pada DB disposable.
7. Uji regresi document/diagram dan akses akun. Jangan deploy VPS atau migrasi
   DB pengguna diam-diam. Nomor/desain migration ditinjau lagi saat implementasi.

### Step 3 - React Preview Dan Generation

1. Tambah pilihan wireframe/UI dan device melalui kontrak backend yang sudah
   aktif; pertahankan pending prompt dan autentikasi yang ada.
2. Implementasikan renderer frame aman, preview multi-frame, zoom/fit, status
   perangkat dan code view. JSON fallback untuk legacy/unsupported version.
3. Uji CSP/sandbox, tidak ada jaringan preview, keyboard, desktop/mobile,
   reduced-motion, serta tidak ada kebocoran session/parent DOM.
4. Tidak redesign homepage/login atau memasang library yang tumpang tindih.

### Step 4 - Revisi Dan Validasi End-To-End

1. Revisi penuh berdasarkan snapshot/version melalui prompt, bukan overwrite
   artifact diam-diam. Pertahankan draft dan idempotency retry yang identik.
2. Tambah scope frame terpilih setelah preservasi frame lain dapat ditegakkan
   pada validator dan commit. Element-level editing ditunda; jangan memakai
   diagram mutation seolah-olah berlaku pada DOM.
3. Uji alur Brainstorm/PRD/diagram -> wireframe -> UI dengan pinned sources,
   termasuk langsung generate UI tanpa memaksa PRD/wireframe lebih dulu.
4. Evaluasi provider Meta asli terpisah setelah mock lulus dan environment siap;
   catat kualitas prompt adherence, repair, biaya/latensi dan batas hasil.

## Gate Dan Status

Sprint 13 masih mempunyai gate uji provider asli yang belum dibuktikan oleh
step ini. Audit Sprint 14 dapat berjalan tanpa menandainya selesai. Login dan
branding yang sudah diverifikasi tidak sama dengan verifikasi generation.

Step 1 selesai bila kontrak/risiko dan urutan implementasi dapat direview.
Step 2 belum selesai dan belum dimulai pada runtime. Tidak ada generator baru
yang ditampilkan sebagai berhasil, tidak ada akun/data yang diubah, dan tidak
ada ketergantungan baru yang perlu diinstal untuk audit ini.
