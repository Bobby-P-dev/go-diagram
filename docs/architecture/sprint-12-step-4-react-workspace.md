# Sprint 12 Step 4: React Workspace

Tanggal: 29 Oktober 2026.

## Posisi dalam sprint

Pekerjaan ini masuk **Sprint 12 Step 4**, bukan sprint baru.
Step 4 dibagi menjadi dua substep agar desain visual dan integrasi backend
bisa direview terpisah:

| Bagian | Lingkup | Status |
| --- | --- | --- |
| Step 4A | Visual foundation, Studio Home, preview contoh, access gate | Sudah diimplementasikan sebelumnya |
| Step 4B | Generation lifecycle React, status live, context trail | Diimplementasikan pada tahap ini |

Sprint 12 Step 1 menyediakan context resolver, Step 2 document engine,
dan Step 3 persisted generation runs serta atomic commit. Step 4 memakai
kontrak tersebut, tidak mengganti schema atau melakukan deployment.

## Step 4A: arah desain

Arah visual: **Soft Architecture Studio**, mengikuti brief clean, elegan,
modern, dan soft. Studio Home bukan salinan frontend Vue lama.

- Warm ivory, forest ink, muted sage, dan aksen chartreuse seperlunya.
- Manrope untuk antarmuka, Instrument Serif untuk editorial,
  IBM Plex Mono untuk data teknis.
- Hover dan reveal ringan; `prefers-reduced-motion` dihormati.
- Preview PRD, ERD, dan UI diberi label sebagai data contoh.
- Referensi yang diminta pengguna: DesainPakeAI, v0, Eraser, dan efek Fastwork.
  Ini arah inspirasi, bukan klaim kesamaan fitur atau hasil audit situs terbaru.

## Step 4B: langkah implementasi

1. Tambahkan tipe generation request, run, status, dan SSE event di React.
2. Hubungkan start, get, cancel, dan authenticated SSE ke API v2.
3. Tambahkan panel generate dari pesan user yang sudah tersimpan.
4. Pilih output Brainstorm/PRD, judul, konteks explicit/project_relevant,
   serta create artifact atau revisi artifact terpilih.
5. Bekukan revision ID sumber dan expected target revision saat memulai.
6. Pertahankan key dan payload yang sama ketika start gagal secara ambigu.
7. Pantau SSE dengan maksimal tiga percobaan per sesi pemantauan; lakukan GET
   untuk rekonsiliasi state saat stream selesai/tersambung kembali.
8. Setelah commit sukses, muat artifact hasil, perbarui daftar artifact,
   dan reload pesan thread. Jangan menampilkan draft inference sebagai hasil.
9. Tampilkan nama dan versi sumber terpin melalui Context Trail.
10. Lindungi draft editor dan dokumentasikan validasi serta batasan.

## Alur pemakaian

1. Masuk memakai credential aplikasi, bukan API key provider model.
2. Pilih project dan thread; kirim instruksi agar tersimpan sebagai pesan.
3. Pilih pesan tersebut pada panel **Susun dokumen dari percakapan**.
4. Pilih Brainstorming atau PRD dan isi judul output.
5. Untuk konteks explicit, pilih artifact di sidebar lalu centang sumbernya.
   Tanpa centang, tidak ada artifact explicit yang dilampirkan.
6. Alternatifnya pilih `project_relevant`; resolver backend memilih konteks
   dalam project yang sama. Ini bukan pencarian lintas project.
7. Centang revisi jika artifact terpilih memiliki tipe yang sama dengan output.
8. Generate; pantau status atau batalkan. Setelah commit hasil dibuka di editor.

## Keamanan dan konsistensi

- SSE memakai `fetch` dan Authorization header, tidak ada token pada URL.
- API key model tetap di backend; tidak ada env rahasia baru di browser.
- Sumber memakai exact revision ID. Navigasi **Buka versi terkini** diberi
  label berbeda karena membuka artifact head, bukan snapshot sumber terpin.
- Generate diblokir ketika editor masih dirty/loading/saving.
- Navigasi project/thread/artifact diblokir ketika run aktif atau start ambigu;
  editor dibuat read-only selama generation. Ada peringatan sebelum tab ditutup.
- Pembatalan bersifat kooperatif. Request cancel tidak dianggap otomatis sukses
  sebelum server mengembalikan status terminal.
- Cleanup membatalkan fetch/stream lokal, tidak membatalkan run server diam-diam.
- Error load hasil tidak mengulang inference; tersedia tombol muat ulang hasil.

## Batasan yang disengaja

- Engine v2 pada tahap ini hanya Brainstorm/PRD. Generation diagram, wireframe,
  dan UI belum disambungkan melalui panel ini.
- Pilihan sumber explicit menggunakan satu artifact terpilih; multi-source
  picker dan pemilihan thread konteks merupakan pengembangan berikutnya.
- Daftar instruksi berasal dari pesan yang sudah dimuat; gunakan pagination
  percakapan untuk menampilkan pesan lain.
- Belum ada auto-resume lintas refresh/login. Run tetap tersimpan di backend,
  tetapi ID run aktif dan credential browser belum dipersistenkan.
- SSE adalah status/heartbeat, bukan token streaming dan bukan persentase.
- Revisi sumber tidak otomatis meregenerasi artifact turunannya.
- Belum ada pengujian kualitas output dengan provider asli pada tahap ini.

## Validasi

- `npm run lint` dan `npm run build` di frontend-react.
- `make verify` dan `make lint-spec` di backend; kontrak 56 operasi cocok.
- Pengujian browser memakai mock, tanpa request ke provider/DB production.
- Vite memberi warning ukuran chunk utama melebihi 500 kB. Build tetap sukses;
  pemisahan bundle dapat dilakukan sebagai tahap optimasi, bukan mematikan warning.

Checklist review berikutnya: coba prompt milik pengguna pada backend development,
periksa ketepatan hasil dan context trail, lalu evaluasi perluasan engine ke diagram
dan UI. Penyelesaian wiring bukan bukti seluruh produk sudah production-ready.
