# Panduan setup — langkah demi langkah

Urutan ini penting: tab rumus merujuk tab lain, jadi tab data dan Form dibuat
lebih dulu. Detail kolom dan rumus lengkap ada di `docs/sheets-schema.md`,
pengaturan Form di `docs/form-mapping.md` — panduan ini menunjuk ke sana.

Perkiraan waktu: 2–3 jam untuk struktur + uji coba, di luar input data lama.

---

## Tahap 0 — Persiapan

1. **Pakai satu akun Google milik pengurus** (bukan akun pribadi) sebagai pemilik
   spreadsheet dan kelima Form. Foto bukti transfer juga masuk ke Drive akun ini.
2. Buat spreadsheet baru, beri nama **Iuran_ClusterN**.
3. **File → Settings** (Setelan):
   - **Locale: United States** — wajib. Dengan locale Indonesia, pemisah argumen
     rumus jadi `;` dan semua rumus di dokumentasi (pakai `,`) akan error saat
     di-paste.
   - **Time zone: (GMT+07:00) Jakarta** — supaya timestamp Form benar.

**Aturan nama tab** — prefix menandai kelompoknya, dan nama harus **persis** seperti
di panduan ini (huruf besar/kecil dan tanda `-` ikut dihitung), karena app dan
rumus memanggil tab berdasarkan nama:

| Prefix | Kelompok | Diisi oleh |
| --- | --- | --- |
| `M-` | Master | diketik manual oleh admin/komite |
| `L-` | Ledger | hanya Google Form — jangan diketik |
| `D-` | Derived | hanya rumus — jangan diketik |

Di dalam rumus, nama tab ber-prefix selalu ditulis dengan tanda kutip satu:
`'M-Rumah'!A2` (semua rumus di dokumentasi sudah begitu).

**Aturan tanggal & waktu** — daftar lengkap tiap kolom ada di
`docs/sheets-schema.md` → *Dates, times and periods*. Yang perlu diingat saat
mengetik:

| Jenis | Cara menulis | Contoh | Dipakai di |
| --- | --- | --- | --- |
| Tanggal | `yyyy-mm-dd` (tahun-bulan-tanggal) | `2026-10-01` | `M-Rumah` tanggal_nonaktif, `M-Opex` diperbarui, `M-SaldoAwal` tanggal, `M-Impor` tanggal_bayar |
| Tanggal + jam (opsional di Impor) | `yyyy-mm-dd hh:mm` (24 jam) | `2025-03-05 14:30` | `M-Impor` tanggal_bayar |
| Tahun | angka 4 digit | `2025` | `M-RumahRiwayat`, `M-TarifVersi` |
| Bulan | angka `1`–`12` (bukan nama bulan) | `6` (Juni) | `M-RumahRiwayat`, `M-TarifVersi`, `M-Impor` |
| Periode | `tahun × 100 + bulan` → `yyyymm` | `202609` (Sep 2026) | rincian Form (diisi app), uji coba Tahap 5 |
| Waktu kiriman | `yyyy-mm-dd hh:mm:ss`, **persis** seperti kolom A `D-Pembayaran`, termasuk detik | `2026-09-16 07:55:00` | Form Keputusan (diisi app) |

Setiap kolom tanggal: blok kolomnya → *Format → Number → Custom date and time* →
ketik `yyyy-mm-dd` → Apply. Wajib untuk `M-Opex` kolom D (app mengurutkan tanggal
itu sebagai teks). Jangan menulis `1/10/2026` atau `01-10-2026` — dengan locale
United States, `1/10/2026` dibaca **10 Januari**, bukan 1 Oktober.

---

## Tahap 1 — Tab master (diisi manual)

Buat tab-tab ini, isi baris 1 dengan header persis seperti di
`docs/sheets-schema.md`, lalu:

| Tab | Yang dikerjakan |
| --- | --- |
| `M-Rumah` | Format kolom **C, D, F** sebagai *Plain text* (Format → Number → Plain text) **sebelum** mengetik. Isi B–F per rumah. Pasang rumus A2, G2, K2, L2 lalu tarik ke bawah sampai semua rumah + cadangan (mis. 300 baris). H–J dibiarkan kosong. |
| `M-RumahRiwayat` | Satu baris baseline per rumah: bulan **pertama** yang ditagih + luas + tipe. Lihat catatan di Tahap 8. |
| `M-TarifVersi` | Satu set baris per versi tarif (format baris: komponen, luas_min, nominal). Kalau tarif pernah berubah selama 3 tahun data lama, masukkan **setiap versi** dengan bulan mulai berlakunya. |
| `M-Blok` | Format kolom A sebagai *Plain text*, isi 10 blok + warna (contoh ada di dokumen). |
| `M-Petugas` | Nama satpam (`satpam`) dan bendahara/admin (`bendahara`). |
| `M-Opex` | Kategori biaya. Format kolom D: *Format → Number → Custom date and time* → `yyyy-mm-dd`. |
| `M-SaldoAwal` | Header dulu; angkanya diisi di Tahap 8. |
| `M-Impor2023` … `M-Impor2026` | Header saja dulu (alamat, bulan, nominal, tanggal_bayar — **tanpa kolom tahun**, tahunnya diambil dari nama tab). Pasang data validation sesuai dokumen. Satu tab per tahun, dari tahun baseline paling awal sampai **tahun go-live** (bulan-bulan sebelum go-live di tahun itu juga masuk sini). Kalau hanya melacak mulai 2026, cukup `M-Impor2026`. |

---

## Tahap 2 — Lima Google Form

Untuk **setiap** Form (detail pertanyaan di `docs/form-mapping.md`):

1. Buat Form, tambahkan pertanyaan **persis sesuai urutan** di dokumen.
2. **Settings → Responses**: *Collect email addresses* → **Do not collect**.
   Untuk Form A, C, D, E: pastikan **tidak** ada "Restrict to users in …" /
   wajib login.
3. Tab **Responses → Link to Sheets → Select existing spreadsheet** →
   Iuran_ClusterN. Google membuat tab baru "Form Responses N" — **rename** sesuai
   tabel di bawah.

| Form | Nama tab hasil |
| --- | --- |
| A. Catat Tunai | `L-Tunai` |
| B. Konfirmasi Transfer (upload bukti di pertanyaan **terakhir**) | `L-Transfer` |
| C. Setor ke Bank | `L-Setoran` |
| D. Pengeluaran | `L-Pengeluaran` |
| E. Keputusan | `L-Keputusan` |

Jangan pernah mengetik atau menaruh rumus di kelima tab ini.

**Folder bukti transfer:** setelah Form B dibuat, Drive otomatis membuat folder
"Konfirmasi Transfer (File responses)". Bagikan folder itu **hanya** ke akun
bendahara (bukan "anyone with the link") — layar Kas menampilkan foto bukti kalau
bendahara sedang login Google di browser yang sama; kalau tidak, ada tombol
"Buka langsung di Drive".

---

## Tahap 3 — Tab rumus

Buat tab-tab ini **setelah** Tahap 1–2 selesai (rumusnya merujuk tab lain), dan
salin rumus dari `docs/sheets-schema.md`:

> **Kalau muncul `#N/A` dengan pesan *"Argument must be a range"*** di rumus yang
> memakai `SUMIFS`/`SUMIF`: hampir pasti ada tab yang dirujuk **belum dibuat**
> (Sheets tidak menampilkan `#REF!` dalam kasus ini). Buat Form-nya dan link ke
> spreadsheet — **jangan** membuat tab kosong sementara dengan nama itu: saat tab
> sementara dihapus nanti, rumus yang merujuknya berubah permanen jadi `#REF!`
> dan harus di-paste ulang.


1. **`D-Pembayaran`** — rumus satu sel di **A1**. Bagian `impor, VSTACK(...)` punya
   satu baris `HSTACK(...)` per tab `M-Impor<tahun>`: hapus baris untuk tahun yang
   tidak Anda buat, atau salin satu baris dan ganti nama tab + angka tahunnya. Menampilkan `#N/A` selama belum ada data:
   itu normal. Format kolom A sebagai teks biasa (sudah teks dari rumus).
2. **`D-Iuran2023` … `D-Iuran2035`** — satu tab per tahun, rumus A1 yang sama, hanya
   angka tahunnya diganti. (Tip: buat `D-Iuran2023`, lalu *Duplicate* dan ubah
   angkanya.)
3. **`D-Pending`** dan **`D-KasMasuk`** — rumus A1 masing-masing.
4. **`D-Riwayat`** — A2:C2 dan A3:B3, tarik ke bawah sampai **baris 121** (120 bulan =
   10 tahun). Baris 2 selalu bulan berjalan dan bergeser sendiri tiap bulan.
5. **`D-API`** — A1:B4 (blok saldo), lalu D1:M1 header dan rumus D2:M2, tarik ke
   bawah sebanyak baris `M-Rumah` (mis. 300).

---

## Tahap 4 — Nonaktif rumah (cukup dicek, tanpa pengaturan tambahan)

Pengamannya ada di rumus `'M-Rumah'!K`: rumah baru nonaktif kalau kolom H berisi
alamatnya **persis**, dan kolom I (oleh) + J (tanggal) terisi. Tidak ada
validasi atau proteksi yang wajib dipasang. Cek sekali di baris rumah mana pun:

| # | Lakukan | Yang harus terjadi |
| --- | --- | --- |
| 1 | Ketik alamat yang salah di H (mis. `N7-9`) | K tetap `TRUE`, L: *"⚠️ nonaktif belum lengkap…"* |
| 2 | Ganti H dengan alamat yang benar, isi I dan J | K jadi `FALSE`, L kosong, `D-API` K juga `FALSE` |
| 3 | **Kembalikan**: hapus isi H, I, J | K kembali `TRUE` |

Pengaman tambahan (validasi, dialog peringatan) opsional — lihat
`docs/sheets-schema.md` → *Menonaktifkan rumah*. Yang lebih berguna: setelah
input data lama selesai, ubah akses komite ke spreadsheet jadi **Viewer**.

---

## Tahap 5 — Uji coba (wajib sebelum input data lama)

Tambahkan satu rumah uji di `M-Rumah` (mis. blok `Blvd`, rumah `99`) + baseline di
`M-RumahRiwayat` bulan ini, luas `100`, tipe `rumah` (tarifnya jadi Rp275.000 dengan
tarif contoh; sesuaikan angka di bawah kalau tarif Anda berbeda). Lalu lakukan, dan cek hasil di `D-Pembayaran`:

| Langkah | Hasil yang diharapkan di `D-Pembayaran` |
| --- | --- |
| Isi Form A langsung: alamat `NBlvd-99`, rincian `202609=275000,202610=275000`, total `550000`, petugas nama satpam | 2 baris, metode `tunai`, keabsahan `sah` |
| Kirim Form A yang sama sekali lagi | 2 baris baru dengan keabsahan `dobel` |
| Isi Form B: rincian `202611=275000`, total `999` + upload foto | 1 baris keabsahan `cek` |
| Isi Form B: rincian `202611=275000`, total `275000` + upload | 1 baris `pending`; muncul juga di tab `D-Pending` |
| Isi Form E: alamat, waktu = kolom A baris transfer tadi, persis termasuk detik (mis. `2026-09-24 10:15:42`), keputusan `sah` | baris itu jadi `sah` |
| Ketik 1 baris di `M-Impor2025`: `NBlvd-99`, 1, 275000 | 1 baris metode `impor`, `sah`, muncul di `D-Iuran2025` |

Cek juga: `'D-API'!L` untuk rumah uji berisi periode yang sah, dan `'D-API'!B2`
(kas_tunai) naik sebesar tunai yang sah.

Kalau ada rumus yang error, kirim screenshot + pesan errornya untuk diperbaiki.
Setelah semua sesuai, **hapus semua baris uji** di `L-Tunai`, `L-Transfer`,
`L-Keputusan`, `M-Impor2025`, `M-RumahRiwayat`, dan baris rumah uji di `M-Rumah`
(boleh menghapus baris di tab Form; yang tidak boleh adalah mengedit isinya
saat sudah dipakai sungguhan).

---

## Tahap 6 — Buka akses baca untuk app

1. **Share** → General access: **Anyone with the link → Viewer**.
2. **File → Share → Publish to web** → *Entire document* → Publish.

Ingat: semua isi spreadsheet ini bisa dibaca siapa pun yang tahu ID-nya — jangan
simpan catatan pribadi (alasan nonaktif, dll.) di sini.

---

## Tahap 7 — Isi `.env` dan coba di komputer

1. Salin `.env.example` → `.env`.
2. `VITE_SHEET_ID` = bagian URL spreadsheet di antara `/d/` dan `/edit`.
3. Untuk tiap Form: buka **Send → link (🔗)** → URL berbentuk
   `https://docs.google.com/forms/d/e/<ID>/viewform` → `<ID>` itu yang diisi ke
   `VITE_FORM_…`.
4. Entry id: di editor Form, **⋮ → Get pre-filled link**, isi semua pertanyaan
   dengan sembarang nilai, **Get link → Copy**. URL-nya berisi
   `entry.123456789=…` untuk tiap pertanyaan, berurutan → isi ke `VITE_E_…`
   sesuai urutan pertanyaan.
5. `VITE_PIN_POS` dan `VITE_PIN_KAS`: PIN layar Pos dan Kas.
6. `npm install && npm run dev`, buka http://localhost:5173 dan coba: buka kartu
   warga, catat tunai di `/pos`, lihat `/kas`. Pastikan baris masuk ke Sheet
   (tunggu 1–5 menit untuk cache gviz).

---

## Tahap 8 — Input data lama

1. Tentukan **tanggal go-live** (disarankan tanggal 1 sebuah bulan). Semua
   pembayaran **sebelum** tanggal itu masuk `M-Impor<tahun>`; sesudahnya lewat app.
2. **`M-RumahRiwayat`**: baseline tiap rumah = bulan pertama yang ditagih (biasanya
   bulan pertama data lama Anda). Setiap bulan sejak baseline yang tidak ada
   pembayarannya akan tampil sebagai tunggakan — jadi baseline dan data impor
   harus sejalan.
3. **`M-Impor2023` … `M-Impor2026`**: satu baris per rumah per bulan yang sudah dibayar
   **sebelum go-live** — termasuk bulan-bulan awal tahun go-live.
   Komite bisa membagi per tahun. Salah ketik cukup diedit langsung di baris itu.
4. **`M-SaldoAwal`**: isi saldo kas tunai dan rekening **pada tanggal go-live**.
5. Cek hasil: buka `/kas` → *Lihat semua kartu rumah* — rumah dengan tunggakan
   yang mencurigakan biasanya berarti ada bulan yang terlewat di impor atau
   baseline yang terlalu awal.

---

## Tahap 9 — Deploy ke GitHub Pages

Lihat `docs/deploy.md` (workflow, secret `DOTENV`, dan pengaturan Pages).

---

## Tahap 10 — Rollout

1. **Petugas**: bagikan link `/#/pos` ke satpam dan `/#/kas` ke bendahara, beserta
   PIN-nya. Tiap orang memilih namanya sekali per HP.
2. **Warga**: dari `/kas` → *Cetak QR per rumah* → cetak dan tempel, atau kirim
   link kartu via WhatsApp dari *Lihat semua kartu rumah*. PIN default = 3 digit
   terakhir no. HP; bisa diganti per rumah di `'M-Rumah'!G`.
3. **Publik**: link `/#/sum` untuk ringkasan kas cluster (tanpa data per rumah).

## Perawatan rutin

- Tarif naik → tambah set baris baru di `M-TarifVersi` (semua komponen, dengan
  bulan mulai berlaku). Jangan edit baris lama.
- Luas/tipe rumah berubah → tambah baris di `M-RumahRiwayat`.
- Rumah baru → baris di `M-Rumah` + baseline di `M-RumahRiwayat`.
- Biaya bulanan berubah → edit `M-Opex` (dan tanggal diperbarui).
- Tab `D-Iuran<tahun>` sudah tersedia sampai 2035 — setelah itu duplikasi lagi.
- Salah edit → **File → Version history** untuk memulihkan.
