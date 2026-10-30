# Sprint 13 Step 4: Targeted Conversational Diagram Revision

Tanggal: 30 Oktober 2026.
Status: implementasi backend dan frontend selesai; pengujian lokal, PostgreSQL disposable, serta browser lintas frontend/backend lolos dengan LLM fixture.

Pengerjaan langsung di backend utama dan frontend React utama. Tidak mengubah Vue, menjalankan deployment, atau memigrasikan database development/project maupun VPS.

## 1. Perilaku

Revisi penuh dari Step 2 tetap tersedia ketika `mutation` tidak dikirim. Mode baru adalah patch UPDATE-only yang secara eksplisit dibatasi ke node/edge pada revisi tertentu. Ini bukan permintaan graph penuh yang sekadar meminta model untuk menjaga elemen lain.

- Mendukung ERD, flowchart, dan sequence; tidak berlaku untuk Markdown, wireframe, atau UI DSL.
- Node terpilih dapat mengubah data label, description, dan fields ERD. Field ERD dapat ditambah/dihapus/direvisi di dalam entity yang dipilih.
- Edge terpilih dapat mengubah label, cardinality ERD, atau message_type sequence sesuai validator tipe diagram.
- Tidak boleh menambah/menghapus/mengurutkan ulang node/edge, mengganti ID/type node, mengubah ID/endpoint/sequence_number edge, atau mengganti judul artifact.
- Semua ID target case-sensitive, harus sudah ada dalam frozen base graph, unik dalam kategorinya, dan totalnya 1..32. Node dan edge boleh memiliki string ID sama karena merupakan kategori berbeda.
- Setiap target harus muncul tepat satu kali dalam replacement arrays model. Minimal satu perubahan efektif diperlukan; no-op tidak membuat revisi semu.
- Operasi struktural di luar batas tersebut tetap memerlukan mode revisi seluruh diagram, bukan perluasan scope otomatis.

## 2. Kontrak API

Route generation v2, autentikasi, SSE, dan Idempotency-Key tidak berubah. Request mendapat properti opsional:

```json
{
  "trigger_message_id": "10000000-0000-4000-8000-000000000004",
  "output_type": "erd",
  "output_title": "Database model",
  "target_artifact_id": "20000000-0000-4000-8000-000000000001",
  "target_expected_revision_id": "20000000-0000-4000-8000-000000000002",
  "mutation": {
    "node_ids": ["users"],
    "edge_ids": []
  },
  "context": {
    "mode": "explicit",
    "sources": []
  }
}
```

UUID dan ID di contoh adalah placeholder kontrak, bukan template output sistem. Kedua array wajib ada; salah satu boleh kosong tetapi tidak boleh keduanya kosong. Mode ini membutuhkan kedua target identifiers dan `output_title` yang persis sama dengan judul artifact tersimpan. Prompt tetap berasal dari pesan user tersimpan, bukan body prompt bebas.

Selektor mutation ikut hash idempotency dan disimpan bersama run. Respons run dapat memuat selektor ID tersebut. Prompt dan kedua representasi base graph tetap private (`json:"-"`); tidak dikirim sebagai field base graph melalui GET/SSE. Frozen manifest tetap mengikuti kontrak konteks yang sudah ada, termasuk source content yang dipilih user dalam project.

Model menghasilkan envelope diagram yang sama, tetapi `graph_nodes` dan `graph_edges` hanya berisi replacement untuk ID terpilih, bukan graph penuh. Server memvalidasi shape JSON secara strict, menolak duplicate/unknown fields, menggabungkan patch dengan frozen typed graph, lalu memvalidasi graph lengkap. Schema prompt membatasi jumlah dan enum ID replacement berdasarkan scope.

Output di luar scope, invalid graph, atau no-op masuk repair loop dengan original input dan target yang sama. Maksimal tiga panggilan keseluruhan; tidak ada template/fallback atau retry tanpa batas. Native constrained decoding provider tidak ditambahkan pada tahap ini.

## 3. Snapshot Dan Commit

1. Penerimaan request memeriksa ownership/project/thread/trigger melalui jalur sebelumnya. Target artifact dikunci dan expected head diperiksa sebelum run dibuat. Target yang tidak ditemukan dalam graph ditolak sebelum inference.
2. Run membekukan selektor dan base graph. Untuk mutation, snapshot JSON juga mempertahankan posisi canvas, metadata, serta properti extension yang tidak ada pada DTO typed. Prompt model tetap memakai representasi typed, bukan kebebasan mengubah properti extension.
3. Engine menggabungkan replacement arrays secara deterministik pada urutan snapshot asli. Coordinator memeriksa lagi hasilnya sebelum commit.
4. Repository commit mengunci run, membaca scope dan snapshot yang tersimpan, lalu memeriksa hasil sekali lagi. Engine/fake candidate tidak dapat melewati batas hanya dengan mengembalikan graph penuh.
5. Perubahan properti yang dikenal di-overlay ke raw snapshot. Elemen tidak terpilih tetap utuh; posisi dan unknown properties node/edge/metadata dipertahankan, termasuk angka JSON yang melampaui presisi float JavaScript. Extension field ERD dipertahankan untuk field dengan nama yang sama; field yang dihapus/berganti nama mengikuti replacement fields user.
6. Expected revision, cancel, head, revision, assistant message, context links dari frozen manifest, active artifact thread, dan status run tetap melalui transaksi atomik sebelumnya. Konflik tidak melakukan overwrite otomatis. Kegagalan setelah insert revision juga me-rollback seluruh transaksi.

## 4. Frontend

Di panel generation: pilih pesan instruksi tersimpan, pilih output yang sesuai artifact, centang revisi artifact, lalu pilih cakupan **Hanya elemen terpilih**. Pilih node/relasi dalam daftar berlabel dan kirim prompt revisi.

- Judul terkunci ke artifact aktif pada mode targeted.
- Pilihan target terikat project/artifact/type/revision; berubah head atau artifact akan mengosongkan pilihan, bukan mengirim target lama. Mode targeted tidak diam-diam berubah menjadi revisi penuh.
- Snapshot tidak tersedia, graph legacy yang tidak sesuai kontrak, tanpa pilihan, atau lebih dari batas target tidak dapat dikirim. Checkbox yang belum dipilih dinonaktifkan setelah 32 target; pilihan aktif tetap dapat dikurangi.
- Dirty/busy editor tetap memblokir generation. Pending/active run mengunci selector; retry ambigu memakai input, mutation, target revision, dan key yang dibekukan pada attempt pertama.
- Inspector canvas tetap baca-saja. Pilihan target dilakukan secara eksplisit dalam panel generation; belum menghubungkan klik Inspector sebagai shortcut.
- Tidak ada perubahan homepage/login, autentikasi, dependency, atau desain ulang workspace.

Dokumentasi konsumen: `../frontend-react/docs/sprint-13-step-4-targeted-revision.md` dan `../frontend-react/docs/TASK_PROGRESS.md`.

## 5. Migration

Migration baru: `internal/platform/database/migrations/07_diagram_mutations.sql`. Menambahkan JSONB `generation_runs.mutation` dan constraint format/snapshot/target/array/count. Migration 04/05/06 tidak ditulis ulang; ledger startup mendaftarkan 07 setelah 06.

Migration hanya dijalankan pada database disposable untuk pengujian. Startup backend versi baru akan menjalankan migration 07 pada database yang dikonfigurasi. Saat aktivasi/deployment nanti, jangan mencampur worker lama dan baru dalam queue yang menerima mutation: hentikan/drain worker lama, jalankan migration melalui startup baru, lalu aktifkan frontend baru. Worker lama tidak mengenali scope dan tidak boleh menerima targeted run.

## 6. File Backend

- `internal/modules/generation/run.go`, `diagram.go`: scope request/run/input engine dan raw snapshot private.
- `internal/modules/generation/diagram_mutation.go`: batas target, server-side apply, dan verifikasi hasil sesuai scope.
- `internal/modules/generation/diagram_snapshot.go`: overlay perubahan typed ke raw snapshot tanpa menghilangkan extension/layout.
- `internal/modules/generation/service/diagram_engine.go`, `diagram_prompt.go`, `diagram_validation.go`: patch schema/prompt, strict parser bersama, merge, repair, dan validasi.
- `internal/modules/generation/service/run_coordinator.go`, `run_generation.go`: validasi mode, copy request selector, penerusan scope, dan recheck candidate.
- `internal/modules/generation/repository/postgres_runs.go`, `postgres_commit.go`: pembekuan scope/raw content dan enforcement di batas transaksi.
- `internal/platform/database/migrations.go`, `migrations/07_diagram_mutations.sql`: registrasi dan migration baru.
- `internal/modules/generation/diagram_mutation_test.go`, `service/diagram_mutation_test.go`: scope, identity/type/order, no-op, patch repair, private data, dan preservation.
- `internal/platform/database/diagram_mutation_integration_test.go`: real HTTP handler/worker/engine dengan mock LLM, persistence, ownership, target validation, replay, scope hash, lineage, konflik, cancel, rollback, dan constraint.
- `internal/platform/database/postgres_test.go`, `connected_workspace_migration_test.go`, `diagram_generation_integration_test.go`: ekspektasi ledger empat migration dan regression upgrade/repeat.
- `docs/api/openapi.yaml`: request/run mutation schema; 56 route tetap.

## 7. Verifikasi Dan Bukti

- `make verify`: PASS (`go vet` dan seluruh Go test offline).
- `make lint-spec`: PASS; OpenAPI 3.0.3, 44 paths, 56 operasi cocok router, 77 schema.
- `go test -race ./internal/modules/generation/... ./internal/integration/... ./internal/app/... -count=1`: PASS.
- `RUN_DATABASE_INTEGRATION_TESTS=1 go test -race ./internal/platform/database -run 'TestDiagramMutationIntegration|TestDiagramGenerationIntegration|TestDiagramMigrationUpgradeIntegration|TestGenerationRunsIntegration|TestConnectedWorkspaceIntegration' -count=1 -v`: PASS pada PostgreSQL 16 disposable. Upgrade dari schema 05 ke 07 mempertahankan data lama dan repeat migration lolos.
- Frontend `npm run lint` tanpa warning lint; `npm run build` PASS. Warning entry bundle >500 kB tetap ada (sekitar 642 kB; renderer diagram sekitar 88 kB dimuat terpisah).
- Chrome desktop 1440x1000 dan mobile 390x844: target picker, keyboard, reduced-motion, batas 32, snapshot berubah/missing, dirty blocking, cancel, retry dengan key/scope identik, revisi penuh tanpa mutation, dan PRD tanpa mode targeted lolos.
- Pengujian lintas proses juga dilakukan: React + WorkspaceApi + HTTP handlers Go + coordinator/worker + diagram engine + PostgreSQL disposable. Ketiga tipe direvisi pada desktop dan mobile (enam kasus), versi tersimpan bertambah dan elemen lain/layout/extensions tetap utuh. Browser mem-forward request ke test server, bukan mengganti respons generation. LLM menggunakan fixture dan auth menggunakan authenticated test context; bukan login/OAuth atau provider nyata.
- Harness dan server sementara sudah dihapus dari project; PostgreSQL/server test dihentikan. Tidak memasukkan bypass atau fixture bisnis ke aplikasi produksi. Bukti sementara di `/tmp/sprint13-step4/` serta `/tmp/sprint13-step4-{verify,race,integration}.log`; tidak dijamin permanen.

## 8. Batas Selesai

Scope implementasi Sprint 13 Step 1..4 kini tersedia untuk generation dan revisi diagram. Hal ini belum merupakan persetujuan deployment atau bukti kualitas inference Meta pada prompt nyata. Auth Google/email-password nyata, evaluasi provider asli, migrasi DB aplikasi/VPS, deployment, renderer wireframe/UI DSL, dan audit lintas browser/performa lebih luas tetap pekerjaan terpisah.
