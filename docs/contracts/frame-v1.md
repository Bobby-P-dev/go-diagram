# Frame V1: Kontrak Target Wireframe Dan UI Design

Tanggal: 30 Oktober 2026
Status: keputusan desain Sprint 14 Step 1; BELUM kontrak runtime aktif.

## 1. Prinsip

`wireframe` dan `ui_design` adalah tipe artifact berbeda dengan format `frame`.
Keduanya menghasilkan static prototype, bukan aplikasi dengan backend atau
JavaScript buatan model. Wireframe fokus struktur, hierarki, label dan alur
dengan fidelity rendah. UI menggunakan warna/typography/detail sesuai brief.
Tidak ada preset industri, pilihan foundation wajib, atau fallback template.

Schema membatasi keamanan dan struktur data, bukan menentukan sidebar, hero,
pricing, dashboard, atau konten bisnis. Layout dan konten berasal dari prompt
dan source yang dipilih. Device shell aplikasi bukan bagian desain hasil AI.

## 2. Input Run

Tetap gunakan endpoint generation v2 dan `Idempotency-Key` yang sama dengan
diagram. Pertahankan trigger message, context selection, output title, target
artifact dan expected revision. Tambahan target: `frame_options` opsional.

- `output_type`: `wireframe` atau `ui_design`.
- `frame_options.device`: `web`, `mobile`, atau `desktop`; default `web`.
- Default canvas awal: web 1024x720, mobile 375x812, desktop 1100x740.
- `frame_options` hanya sah untuk output frame, bukan diam-diam diabaikan pada
  dokumen/diagram. Options yang dinormalisasi masuk request hash dan frozen run.
- Lebar/tinggi actual tiap frame boleh disesuaikan engine dalam limits jika
  brief memerlukan halaman panjang. Explicit device option mengikat output.
- `mutation` diagram tidak berlaku. Pada Step 2 hanya generate/revisi penuh;
  scope frame terpilih baru diperluas secara eksplisit pada Step 4.
- Tidak menaruh API key, HTML, base snapshot, author ID atau source content
  yang dipercaya server langsung dari request klien.

Revisi mengambil `base_frame` dari revision target milik project/user yang sah,
dibekukan saat run diterima. Konflik head saat commit menggagalkan penyimpanan,
bukan menimpa versi baru. Revisi mempertahankan ID frame yang sama jika frame
tersebut tetap ada. Penambahan/penghapusan hanya melalui revisi penuh eksplisit.

## 3. Envelope Candidate

Engine mengembalikan `type`, `title`, `format: frame`, `schema_version: 1`,
`frame_data`, `change_summary`, dan `assistant_message`. Source refs disusun
server dari frozen manifest, bukan dipercaya dari ID yang dikarang model.
Document/graph root payload harus kosong. Hanya candidate valid dapat commit.

## 4. Canonical Frame Data

Contoh berikut fixture kontrak, BUKAN template yang disuntikkan ke generation.

```json
{
  "schema_version": 1,
  "kind": "wireframe",
  "nodes": [
    {
      "id": "frame-1",
      "type": "ui_frame",
      "position": { "x": 0, "y": 0 },
      "data": {
        "canvas": {
          "title": "Halaman contoh",
          "device": "web",
          "width": 1024,
          "height": 720
        },
        "design": {
          "intent": "Memperlihatkan struktur halaman sesuai brief.",
          "requirements": ["Ada judul halaman"],
          "decisions": ["Hierarki satu kolom untuk contoh kontrak."]
        },
        "implementation": {
          "html": "<main><h1>Judul halaman</h1></main>",
          "css": "main { padding: 24px; color: #1f2937; background: #ffffff; }"
        }
      }
    }
  ],
  "edges": []
}
```

- Root wajib tepat `schema_version`, `kind`, `nodes`, `edges`; kind harus sama
  dengan artifact type/run output type. Edge array kosong pada v1; navigasi
  antar layar belum diklaim sebagai executable interaction.
- Canonical node wajib tepat `id`, `type`, `position`, `data`; type `ui_frame`.
  ID unik dalam artifact, 1..64 ASCII huruf/angka/underscore/hyphen.
- `data` wajib tepat `canvas`, `design`, `implementation`. Tidak menyimpan
  `raw_prompt`, `raw_html`, `code_export`, atau duplicate legacy alias di sini.
- Canvas wajib title nonblank <=255 karakter, device enum di atas, width
  integer 240..2560 dan height integer 240..12000. Position finite dalam
  -100000..100000. Lebar canvas logis bukan alasan document mobile overflow.
- Design wajib intent nonblank <=2048 karakter, requirements 1..64 strings,
  decisions 0..64 strings; tiap item nonblank <=1024 karakter. Pernyataan ini
  bukan bukti seluruh kebutuhan sudah terimplementasi; perlu pemeriksaan hasil.
- Implementation wajib HTML fragment nonblank dan CSS string (boleh kosong).
  Tidak ada field JS, framework source, remote imports, atau kode executable.
- Unknown keys ditolak untuk payload canonical v1, termasuk nested objects.
  Future version perlu adapter/validator tersendiri; jangan diam-diam strip
  extension dari data legacy lalu menyimpannya sebagai v1.
- Minimal 1, maksimal 8 frames per generation; maksimal 1 MiB UTF-8 encoded
  frame_data, HTML <=128 KiB/frame dan CSS <=32 KiB/frame. Tolak NUL, invalid
  UTF-8, angka nonfinite dan duplicate JSON object keys. Parser juga dibatasi
  nesting 32 levels dan HTML 5000 elements/frame sebelum render.
- Revision `schema_version` yang sudah ada tetap 1. Marker nested frame v1
  membedakan kontrak ini dari object frame legacy tanpa marker; bukan alasan
  menaikkan semua schema revision atau merombak existing artifacts.

## 5. HTML, CSS Dan Preview Safety

Ini persyaratan implementasi dan pengujian, bukan klaim sanitizer lama sudah
memenuhi semuanya. Valid JSON tidak berarti HTML aman.

- HTML diparse, bukan divalidasi regex saja. Gunakan allowlist static semantic
  HTML: main/section/article/header/footer/nav/aside/div/span, headings h1..h6,
  p/br/hr, lists, table elements, form/label/input/textarea/select/option/button,
  details/summary, strong/em/small dan pre/code. A hanya untuk local fragment
  yang tidak berpindah halaman. Atribut juga allowlist: presentational identity,
  ARIA, safe input/display attributes; tanpa on*, directives, srcdoc, autofocus,
  URL-bearing attributes atau form actions. Kontrol tidak boleh submit data.
- Asset jaringan, script, iframe, object, embed, custom elements, external font,
  arbitrary SVG dan remote images tidak masuk v1. Pakai CSS/local text untuk
  placeholder visual; tidak menambah gambar stok secara otomatis. Dukungan asset
  milik aplikasi dapat diperluas dengan kontrak allowlist terpisah nanti.
- CSS diparse dengan allowlist property/value/at-rule, batas kompleksitas, dan
  penolakan import, URL/network references, executable expressions, escape-based
  bypass serta active browser-specific behavior. Izinkan layout, typography,
  warna, spacing, borders dan media queries yang aman. Jangan mengeksekusi
  Tailwind CDN atau arbitrary class compilation saat membuka preview.
- Generation baru mengeluarkan CSS self-contained. Legacy Tailwind HTML tidak
  boleh dianggap ekuivalen atau di-render tanpa adapter. Jika konversi perlu
  compiler lokal, evaluasi terpisah; jangan runtime-fetch dependency dari CDN.
- Preview React memakai iframe srcdoc sandbox tanpa allow-scripts,
  allow-same-origin, forms, popups, downloads atau top-navigation. Parent yang
  membuat wrapper, CSP, base styles dan viewport, bukan model.
- CSP target: default-src 'none'; script-src 'none'; connect-src 'none';
  img-src 'none'; font-src 'none'; media-src 'none'; object-src 'none';
  frame-src 'none'; base-uri 'none'; form-action 'none'; style-src 'unsafe-inline'.
  Jangan menyisipkan CSS/HTML dengan string concatenation yang bisa menutup
  style/document boundary tanpa escaping/serialization yang tepat.
- Render dan code export memakai hasil safety pipeline yang sama; jangan
  menyajikan original unsanitized output di code export. Simpan snapshot yang
  sudah diverifikasi, bukan output parsial streaming yang belum valid.
- Perubahan sanitizer yang menghilangkan konten wajib membuat hasil diperiksa
  lagi. Repair maksimal satu; jika konten/fidelity tetap invalid, run gagal
  tanpa artifact sukses palsu. Jangan menjanjikan preview interaktif aplikasi.

## 6. Context Dan Fleksibilitas

Gunakan manifest existing: context mode, exclusions, budget dan sumber revision
yang dipin. PRD, brainstorming, ERD/flowchart/sequence atau wireframe boleh jadi
referensi, tetapi tidak ada urutan wajib. Tanpa sumber, generate dari prompt
dan project description yang diizinkan; jangan pura-pura sudah membaca PRD.

Instruksi user utama mengarahkan perubahan. Source, HTML, history dan previous
invalid output diperlakukan sebagai data tidak tepercaya. Konflik kebutuhan
harus dijelaskan melalui summary/assistant message, bukan menambahkan fitur
bisnis diam-diam. Tidak otomatis meregenerasi artifact downstream.

API auth/ownership tetap sama. Prompt/context penuh tidak dicetak ke log atau
preview login. Cross-project refs ditolak server. Jangan mengambil konteks
project lain meskipun model meminta. Tautan source dibuat dari manifest dalam
transaksi yang sama dengan revision baru.

## 7. Legacy Dan Kompatibilitas

Jangan bulk-convert `projects.current_nodes` atau revision frame lama pada step
ini. Endpoint `/api/ui-design/*` lama tetap terpisah selama transisi.

Adapter opt-in yang kelak ditulis harus memetakan Canvas/ImplementationState
dan alias legacy, mengecek konflik antar alias, serta menolak payload yang tak
bisa dikonversi dengan aman. Membuka artifact tidak boleh menulis ulang revision.
React menyediakan JSON/read-only fallback untuk unversioned/unknown schema;
pengguna tidak menerima blank preview yang tampak seperti sukses.

## 8. Matriks Penerimaan Step 2-4

| Kasus | Hasil wajib |
| --- | --- |
| Wireframe dan UI valid, satu/multi frame | Snapshot frame + revision + message + links + run success atomik |
| Tidak ada PRD/wireframe sebelumnya | Boleh generate dari prompt, tidak memaksa wizard |
| Duplicate ID/key, wrong kind, invalid device, unknown fields, oversize | Ditolak; tidak ada partial artifact |
| Prompt meminta script/network/form submission | Tidak dieksekusi, tidak bocor data; error/repair yang jujur |
| HTML/CSS malicious, boundary escape, external URL | Ditolak/disanitasi dan diverifikasi; network preview tetap nihil |
| Malformed output lalu repair valid | Maksimal dua call, hasil akhir saja disimpan |
| Repair gagal/provider timeout/cancel | Run gagal/canceled sesuai lifecycle; tidak ada revision sukses |
| Expected revision stale | Konflik; draft dan head baru tidak ditimpa |
| Retry idempotent dan restart lifecycle | Tidak menggandakan artifact/message; aturan recovery existing tetap berlaku |
| Source milik project/account berbeda | Ditolak sebelum provider dipanggil |
| Legacy/unknown frame dibuka | Fallback jelas, tidak execute atau mengubah data otomatis |
| Diagram/Markdown dan login existing | Perilaku/API yang sudah tersedia tetap kompatibel |
| Desktop/mobile/keyboard/reduced motion | Preview terisolasi, toolbar aksesibel, tanpa document overflow |

Gate selesai bukan sekadar schema lolos: safety dan lifecycle diuji dengan mock
dan PostgreSQL disposable; kualitas visual/provider asli diuji terpisah setelah
alur aman. Tidak ada klaim seluruh gate sudah lulus pada Step 1.
