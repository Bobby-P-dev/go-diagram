# Sprint 13 Step 2: Diagram Lifecycle And Atomic Persistence

Tanggal: 29 Oktober 2026.
Status: implementasi backend dan validasi offline/integration selesai.
Pengerjaan langsung di `/var/www/project-diagram/backend-diagram`, bukan worktree. Tidak mengubah frontend atau menjalankan deployment.

## 1. Yang Diimplementasikan

- Endpoint generation v2 yang sudah ada kini menerima `brainstorm`, `prd`, `erd`, `flowchart`, dan `sequence`.
- Coordinator memilih document engine untuk Markdown dan diagram engine untuk graph. Keduanya memakai client/gateway LLM yang sudah tersedia dan dua worker generation yang sama, bukan queue atau worker pool baru.
- Domain `ArtifactCandidate` menjadi kontrak commit bersama. Engine document dan diagram tetap memiliki kontrak terpisah; adapter service mengubah hasil menjadi candidate yang divalidasi ulang sebelum persistence.
- Validasi integritas graph dan batas resource dipindahkan dari service ke domain generation agar engine dan repository menggunakannya tanpa repository mengimpor service.
- String output yang mengandung NUL ditolak sebelum persistence, termasuk pada graph, sehingga masuk validasi/repair alih-alih gagal saat encoding PostgreSQL JSONB/text.
- Run revisi membekukan `base_document` atau `base_graph` dari target revision saat request diterima. Prompt berasal dari pesan user tersimpan, bukan string prompt arbitrer dari request.
- Graph disimpan ke `artifact_revisions.graph_nodes` dan `graph_edges`; document content tetap SQL NULL untuk graph. `metadata.diagram_type`, schema version, trigger, author dan change summary ikut tersimpan.
- Artifact/revision, head, assistant message, active artifact thread, exact source links, dan status succeeded run disimpan dalam satu transaksi.

## 2. Kontrak API

Tidak menambah route. Tetap memakai:

```text
POST /api/v2/projects/{projectId}/threads/{threadId}/generations
GET  /api/v2/projects/{projectId}/generations/{runId}
POST /api/v2/projects/{projectId}/generations/{runId}/cancel
GET  /api/v2/projects/{projectId}/generations/{runId}/events
```

Contoh body untuk generate ERD dari pesan user dan revision PRD yang dipilih:

```json
{
  "trigger_message_id": "10000000-0000-4000-8000-000000000004",
  "output_type": "erd",
  "output_title": "Library ERD",
  "context": {
    "mode": "explicit",
    "sources": [{
      "artifact_id": "20000000-0000-4000-8000-000000000001",
      "revision_id": "20000000-0000-4000-8000-000000000002"
    }]
  }
}
```

UUID di contoh hanya placeholder dokumentasi, bukan data/template output. Request tetap memerlukan autentikasi aplikasi dan `Idempotency-Key`. Untuk revisi tambahkan pasangan `target_artifact_id` dan `target_expected_revision_id`. Target harus cocok tipe/formatnya dan berada dalam project yang sama. `sequence` adalah nama kanonis, bukan `sequence_diagram`; wireframe/UI belum diaktifkan dalam generation v2.

Respons accepted, GET, cancel, SSE status/heartbeat/complete, dan identitas committed artifact/revision/message tidak berubah. Private `prompt`, `base_document`, dan `base_graph` tidak masuk JSON run, termasuk SSE. Hasil graph diambil melalui detail artifact/revision yang sudah tersedia setelah run berhasil, bukan melalui delta inference.

## 3. Konsistensi Dan Keamanan

1. Scope owner/project/trigger diperiksa melalui jalur yang sudah ada; target revision dibaca sambil mengunci artifact saat snapshot dibuat.
2. Idempotency tetap memakai hash request dan scope user/project/thread. Replay mengembalikan run yang sama; request berbeda dengan key sama tetap konflik.
3. Frozen manifest menjadi sumber otoritatif lineage. Repository tidak memakai klaim provenance dari kandidat model untuk membuat context links.
4. Saat commit, run dikunci; cancel yang lebih dulu tercatat menang tanpa menyimpan artifact baru. Status terminal tidak dapat dikomit ulang.
5. Revisi target dikunci dan dibandingkan lagi dengan expected revision. Salah satu dari dua run terhadap head yang sama akan gagal dengan revision conflict setelah run lain memajukan head.
6. Kegagalan revision/message/context-link membuat seluruh transaksi rollback, termasuk perubahan head dan active artifact thread. Status succeeded hanya ditulis pada transaksi yang berhasil.
7. Tidak ada transaksi database yang ditahan selama panggilan LLM. Cancellation kooperatif, lease/recovery, timeout, dan sanitasi pesan error tetap melalui lifecycle yang sama.

Diagram repair yang habis dipetakan ke `REPAIR_FAILED`, response terlalu besar ke `DIAGRAM_TOO_LARGE`, dan engine tidak tersedia ke `ENGINE_UNAVAILABLE`. Provider error lain menjadi `GENERATION_FAILED`; kegagalan transaksi menjadi `COMMIT_FAILED` atau `REVISION_CONFLICT`. Tidak ada fallback diagram/template palsu.

## 4. Migration

Migration baru: `internal/platform/database/migrations/06_diagram_generation.sql`.

- Memperluas CHECK output type menjadi lima tipe yang didukung.
- Menambahkan `generation_runs.base_graph JSONB` sebagai private frozen snapshot.
- Memastikan graph snapshot berupa object, document/graph tidak bercampur, dan target revisi memiliki snapshot sesuai tipenya. Run baru tanpa target tidak boleh membawa base snapshot.
- Didaftarkan setelah migration 05. Migration 04/05 tidak diedit sehingga checksum lama tetap berlaku.

Migration diuji pada PostgreSQL lokal **disposable** untuk fresh install, upgrade dari schema 05 dengan run PRD yang sudah ada, dan pemanggilan berulang. Data PRD lama tetap ada. Tidak menjalankan migration terhadap database development project atau VPS pada pengerjaan ini.

Entry point backend sudah menjalankan `RunMigrations` saat startup; binary baru memerlukan schema 06 sebelum melayani generation. Pada deployment nanti, hentikan worker binary lama sebelum mengaktifkan request diagram, bukan mencampur worker lama dan baru pada queue yang sama. Tidak perlu mengirim API key ke chat atau mengubah env provider pada tahap ini.

## 5. File

- `internal/modules/generation/candidate.go` dan `candidate_test.go`: kontrak commit umum, pemetaan output format, dan validasi candidate.
- `internal/modules/generation/diagram_validation.go`: validator graph domain bersama.
- `internal/modules/generation/engine.go` dan `run.go`: pesan error umum, base graph private, dan port commit candidate.
- `internal/modules/generation/service/run_generation.go`: dispatch engine dan adapter candidate.
- `internal/modules/generation/service/run_coordinator.go`: penerimaan tipe diagram dan pemetaan error lifecycle.
- `internal/modules/generation/service/diagram_engine.go` dan `diagram_validation.go`: penggunaan validator/batas domain, tanpa menduplikasi aturan graph.
- `internal/modules/generation/repository/postgres_runs.go`: snapshot/scan base graph dan penerimaan run.
- `internal/modules/generation/repository/postgres_commit.go`: transaksi commit bersama document/graph.
- `internal/app/app.go`: wiring kedua engine pada client LLM yang sama.
- `internal/platform/database/migrations.go` dan `migrations/06_diagram_generation.sql`: registrasi dan perubahan schema baru.
- `docs/api/openapi.yaml`: enum dan penjelasan kontrak generation diperbarui; tetap 56 operasi dan 44 path.
- `internal/modules/generation/service/run_diagram_test.go`: dispatch, snapshot input worker, error mapping, dan enabled/unsupported types.
- `internal/modules/generation/service/diagram_validation_test.go`: regression penolakan escaped NUL pada label graph.
- `internal/modules/generation/controller/run_controller_test.go`: regression GET/SSE dan pengujian snapshot private tidak terekspos.
- `internal/modules/generation/service/run_coordinator_test.go` dan `internal/platform/database/generation_runs_integration_test.go`: penyesuaian constructor/commit contract tanpa mengubah tujuan regression tests dokumen.
- `internal/platform/database/diagram_generation_integration_test.go`: HTTP acceptance, real worker/engine dengan mock LLM, graph persistence, revisi, lineage, konflik, cancel, rollback, dan upgrade schema.
- `internal/platform/database/postgres_test.go` dan `connected_workspace_migration_test.go`: ledger migration 06 dan jumlah migration diperbarui.
- `docs/architecture/sprint-13-step-1-diagram-generation.md` dan dokumen ini: penanda status historis Step 1 serta hasil Step 2.

Tidak menambah dependency, environment variable, credential, route baru, atau perubahan frontend.

## 6. Verifikasi

- `make verify`: PASS, mencakup `go vet ./...` dan seluruh suite Go offline.
- `make lint-spec`: PASS; 56 operasi cocok dengan router, termasuk enum output diagram.
- `go test -race ./internal/modules/generation/... ./internal/platform/database/... ./internal/integration/... ./internal/app/... -count=1`: PASS untuk suite biasa; database integration opt-in dijalankan terpisah.
- `RUN_DATABASE_INTEGRATION_TESTS=1 go test -race ./internal/platform/database -run 'TestDiagramGenerationIntegration|TestDiagramMigrationUpgradeIntegration|TestGenerationRunsIntegration|TestConnectedWorkspaceIntegration' -count=1 -v`: PASS pada PostgreSQL 16 disposable, bukan database aplikasi.
- Ketiga output diuji melalui HTTP handler ber-authed context, coordinator/worker sungguhan, engine dengan mock LLM, serta database sungguhan. Memverifikasi graph-only content, metadata, message, source revision, idempotency, snapshot revisi, optimistic conflict, dan cancel sebelum commit.
- Trigger kegagalan context-link dipakai untuk membuktikan rollback graph baru maupun revisi graph: tidak ada revision/message parsial dan head lama tidak berubah.
- Upgrade dari migration 05 mempertahankan run PRD yang sudah ada dan tetap idempotent; constraint menolak tipe yang belum didukung, document snapshot pada graph run, dan snapshot non-object.
- Bukti sementara ada di `/tmp/sprint13-step2-verify-final.log` dan `/tmp/sprint13-step2-integration-final.log`; bukan artefak permanen project.

Pengujian HTTP memakai authenticated test context, bukan OAuth/login sungguhan. Tidak memakai provider asli, API key, atau browser frontend. JSON Schema masih diberikan pada system prompt dan diverifikasi lokal; native provider constrained decoding belum ditambahkan.

## 7. Berikutnya

Step 3: tambahkan pilihan diagram pada generation panel React, sinkronkan TypeScript types, dan kembangkan renderer/inspector ERD, flowchart, serta sequence sesuai kontrak graph. Pertahankan visual yang sudah disepakati. Belum mengerjakan Step 3, auth, targeted partial mutation Step 4, atau deployment pada tahap ini.
