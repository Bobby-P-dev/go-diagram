# Sprint 13 Step 1: Diagram Generation Engine

Tanggal: 29 Oktober 2026.
Status: implementasi kontrak, engine, parser/validator, dan pengujian mock selesai.

Pembaruan: integrasi lifecycle/persistence kini dikerjakan pada Step 2. Lihat `sprint-13-step-2-diagram-lifecycle.md` untuk status terbaru; batas endpoint/persistence di dokumen Step 1 ini adalah catatan historis tahap tersebut. Validator graph bersama dipindahkan ke domain `generation/diagram_validation.go` agar engine dan repository memakai aturan yang sama.
Lokasi pengerjaan: project utama `/var/www/project-diagram/backend-diagram`, bukan worktree.

## 1. Lingkup Dan Batas Tahap

Sprint 12 menyediakan context resolver, document engine Brainstorm/PRD, generation runs, dan integrasi React. Refinement homepage/login setelahnya merupakan pekerjaan visual tambahan; autentikasi Google/email masih UI preview.

Step 1 ini menambah engine diagram terpisah di modul generation. Belum mengubah route, worker/coordinator, repository commit, OpenAPI, database, frontend, atau deployment. Diagram belum tersedia melalui endpoint generation v2 yang saat ini tetap memakai document engine. Jangan menganggap dokumen ini sebagai bukti fitur diagram sudah terhubung end-to-end.

## 2. Kontrak Domain

`generation.DiagramEngine.GenerateDiagram` menerima `DiagramInput`:

- `type`: tepat `erd`, `flowchart`, atau `sequence`. Nama kanonis sequence adalah `sequence`, bukan `sequence_diagram`.
- `title`: judul opsional, maksimal 255 karakter; jika diisi, output harus mempertahankan judul setelah trim.
- `prompt`: instruksi pengguna, wajib, maksimal 16 KiB.
- `manifest`: frozen context manifest dari resolver yang sudah ada, termasuk revision sumber dan percakapan.
- `base_graph`: graph opsional untuk revisi penuh; harus valid dan memiliki tipe yang sama.

Hasilnya `DiagramCandidate`: `type`, `title`, `graph_nodes`, `graph_edges`, `metadata`, `change_summary`, `assistant_message`, `format=graph`, `schema_version=1`, dan exact revision `sources`.

Node memakai `id`, `type`, dan `data.label`; `data.description` opsional. Edge memakai `id`, `source`, `target`, serta properti spesifik jenis diagram. Metadata saat ini hanya `diagram_type`.

| Jenis | Node | Properti Khusus |
| --- | --- | --- |
| ERD | `entity` | `data.fields`: nama, tipe data, primary key, nullable; edge memiliki cardinality `1:1`, `1:N`, `N:1`, atau `N:M` relatif terhadap source lalu target |
| Flowchart | `start`, `process`, `decision`, `end` | Edge keluar decision wajib punya label cabang; tidak memaksakan jumlah cabang atau struktur template |
| Sequence | `participant` | Edge berupa pesan dengan label, `sequence_number` berurutan 1..N sesuai urutan array, dan `message_type` berupa `call`, `return`, atau `async` |

Node IDs dan edge IDs harus unik dalam kategorinya, memakai pola ASCII `^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`. Semua endpoint harus menunjuk node yang ada. Field ERD tidak boleh memiliki nama duplikat; primary key tidak boleh nullable. Self-relation, self-call, dan loop diperbolehkan. ERD/flowchart boleh tanpa edge, sedangkan sequence wajib memiliki minimal satu pesan.

Tidak memasukkan posisi canvas, CSS, atau layout engine ke hasil LLM. Layout dan pemetaan node renderer adalah urusan frontend pada tahap berikutnya. Model menentukan isi graph dari kebutuhan, bukan memilih template diagram hardcoded.

## 3. Pipeline Dan Structured Output

1. Periksa cancellation, client, tipe input, prompt, manifest, base graph, dan total input budget.
2. Serialize envelope asli dengan frozen manifest. Tidak mengambil history tersembunyi atau membaca project lain.
3. Bangun system prompt dan JSON Schema spesifik jenis diagram, tanpa contoh diagram atau daftar node contoh.
4. Panggil port `LLMClient` yang sudah dipakai document engine. Provider/key/config tidak diubah.
5. Parse tepat satu JSON object. Code fence tunggal diperbolehkan; prose tambahan, trailing JSON, unknown field, duplicate key termasuk nested/escaped key, null, dan tipe data salah ditolak.
6. Validasi struktur dan integritas graph, semantic constraint per tipe, judul, serta batas ukuran.
7. Bila tidak valid, kirim original input, invalid response, dan alasan validasi sebagai data JSON ke repair loop. Maksimal tiga panggilan total, bukan tiga repair tambahan. Tidak menumpuk seluruh history repair.
8. Kembalikan kandidat hanya setelah valid. Jika tetap gagal, kembalikan `ErrDiagramRepairFailed`, tanpa fallback diagram palsu.

JSON Schema diberikan dalam system prompt dan diverifikasi dengan parser/validator lokal. Port LLM saat ini belum mengirim native provider `response_format/json_schema`; tahap ini tidak mengklaim constrained decoding provider sudah aktif. Kontrak lokal tetap menjadi batas penerimaan hasil.

Sumber PRD/Brainstorm, deskripsi project, history, base graph, dan invalid response diposisikan sebagai reference data tidak tepercaya. PRD tidak wajib agar alur tetap fleksibel. `sources`, format, dan schema version diturunkan server-side, bukan dipercayakan kepada output model. Instruksi prompt bukan jaminan bahwa model kebal prompt injection; pembatasan scope tetap tanggung jawab resolver/coordinator server.

## 4. Batas Resource Dan Error

- Total input JSON maksimal 256 KiB.
- Raw response maksimal 512 KiB; response yang melebihi batas langsung gagal, tidak diperbaiki.
- Serialized graph maksimal 256 KiB; maksimal 200 node, 500 edge, dan 100 field per entity.
- Nesting JSON maksimal 32 level.
- Judul/node label maksimal 255 karakter; description maksimal 2048 byte, summary maksimal 2048 byte, assistant message maksimal 4096 byte. Field names dan data types maksimal 128 karakter.
- Cancellation sebelum/sesudah inference diperiksa; transport/provider error diteruskan tanpa dianggap sebagai kegagalan format dan tanpa schema repair tambahan.
- Callback delta diteruskan melalui port, tetapi delta belum tervalidasi dan bukan artifact yang boleh di-commit. Batas raw response engine diperiksa setelah client mengembalikan output.

Tidak ada repository write atau commit lineage pada tahap ini. `candidate.Sources` berisi revision refs yang dibekukan dari manifest; pemakaian dalam transaksi artifact/revision adalah Step 2.

## 5. Struktur File

- `internal/modules/generation/diagram.go`: tipe domain, candidate, error, dan port DiagramEngine.
- `internal/modules/generation/service/diagram_engine.go`: orkestrasi inference, repair, cancellation, dan candidate.
- `internal/modules/generation/service/diagram_prompt.go`: prompt dan JSON Schema per jenis diagram.
- `internal/modules/generation/service/diagram_validation.go`: strict JSON parser dan validasi graph.
- `internal/modules/generation/service/engine_manifest.go`: ekstraksi validator manifest yang sudah dipakai document engine, untuk digunakan bersama tanpa menduplikasi logika.
- `internal/modules/generation/service/document_engine.go`: hanya mengganti blok validasi manifest dengan pemanggilan helper bersama; perilaku document engine dipertahankan.
- `internal/modules/generation/service/diagram_engine_test.go`: mock generation, konteks PRD, lineage, revisi, repair, limit input, error, streaming callback, dan cancellation.
- `internal/modules/generation/service/diagram_validation_test.go`: strict parsing, integritas graph, aturan tiap diagram, batas resource, dan fuzz parser.
- `docs/architecture/sprint-13-step-1-diagram-generation.md`: status dan hasil tahap ini.

Tidak menambah dependency, environment variable, migration, route, atau template hasil.

## 6. Validasi

- `go test ./internal/modules/generation/... -count=1`: PASS, termasuk regression document engine dan architecture boundary tests.
- `make verify`: PASS (`go vet ./...` dan seluruh suite Go).
- `make lint-spec`: PASS; kontrak yang sudah ada tetap 56 operasi cocok dengan router. Tidak ada endpoint baru pada Step 1.
- `go test -race ./internal/modules/generation/... -count=1`: PASS.
- `go test ./internal/modules/generation/service -run '^$' -fuzz '^FuzzParseDiagram$' -fuzztime=3s -parallel=2`: PASS; 56.744 eksekusi pada run yang dicatat, tanpa crash.
- Tes positif mencakup ketiga tipe, konteks PRD lengkap dan revision refs yang sama, revisi base graph, tanpa prasyarat PRD, single code fence, self-relation, serta batas tepat jumlah node/edge/field.
- Tes negatif mencakup dangling edge, duplicate node/edge/field/key, spoofed provenance, unknown/case-alias field, null, boolean yang hilang/salah tipe, metadata tidak cocok, cardinality, urutan pesan, ukuran output/input/graph, serta repair yang habis tanpa fallback.

Semua menggunakan mock/offline; tidak memanggil provider asli, memakai API key, mengubah data VPS, atau memigrasikan database. Validasi semantik tidak membuktikan ketepatan semua kebutuhan bisnis; kualitas grounding hasil model nyata masih perlu diuji setelah integrasi.

## 7. Sisa Sprint 13

1. Step 2: sambungkan engine diagram ke lifecycle generation yang sudah ada, persistence graph/revision/lineage secara atomik, dan kontrak API/OpenAPI. Jangan membuat worker paralel baru tanpa kebutuhan.
2. Step 3: hubungkan pilihan output React dan renderer/inspector yang sesuai jenis diagram. GraphEditor dasar sudah ada; perlu dikembangkan, bukan diasumsikan belum ada sama sekali.
3. Step 4: targeted conversational revision serta pengujian end-to-end desktop/mobile. Base graph di Step 1 mendukung kandidat revisi penuh, belum menerapkan mutasi parsial terotorisasi.
