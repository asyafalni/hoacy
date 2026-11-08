# Panduan setup — langkah demi langkah

Urutan: akun-akun (Tahap 0–3) → coba di komputer (Tahap 4, opsional) → deploy
(Tahap 5, detail di `docs/deploy.md`) → pindah data dan rilis (Tahap 6).

**Aturan rahasia:** connection string database, Clerk *secret key*, dan token
Google **tidak pernah** ditempel di chat, issue, atau file yang di-commit.
Tempatnya hanya `backend/.env` di komputer Anda (sudah di-.gitignore) dan env var
terenkripsi di DigitalOcean.

---

## Tahap 0 — Yang perlu disiapkan

| Akun | Untuk apa |
| --- | --- |
| GitHub (repo `asyafalni/hoacy`) | sumber kode, DigitalOcean deploy dari sini |
| Aiven (Postgres, region DigitalOcean `blr`) | database |
| Clerk | login pengurus (admin / bendahara / komite) |
| **Akun Google khusus RT**, mis. `iuran.cypress@gmail.com` | menyimpan foto bukti di Drive + membaca Sheet untuk sinkron |
| DigitalOcean | hosting (App Platform, region `blr`) |
| Domain milik sendiri | wajib untuk Clerk *production* dan sebelum QR dicetak |

Di komputer (untuk Tahap 4 dan `google-login`): Go ≥ 1.27.3, Node 22, Docker.

## Tahap 1 — Database (Aiven)

1. Aiven → service Postgres → **Overview → Service URI**. Bentuknya
   `postgres://avnadmin:…@….aivencloud.com:…/defaultdb?sslmode=require`.
   Database `cypressdb` boleh dipakai — ganti nama di ujung URI.
2. Kalau password pernah terlihat orang lain (mis. tertempel di chat): **Users →
   avnadmin → Reset password**, lalu pakai URI yang baru.
3. Skema dibuat otomatis saat server pertama kali jalan (migrasi). Tidak ada
   tabel yang perlu dibuat manual.

## Tahap 2 — Clerk (login pengurus)

1. clerk.com → **Create application** → nama "Iuran Cypress". Pilihan masuk:
   **hanya Google** — matikan Email, Password, dan cara lain
   (**Configure → User & authentication**).
2. **Configure → Restrictions → Sign-up mode: Restricted.** Tidak ada yang bisa
   daftar sendiri. Superadmin mendaftarkan setiap pengurus:
   - **Clerk → Users → Create user** dengan alamat Gmail pengurus. Saat dia
     pertama kali *Sign in with Google* dengan Gmail yang sama, Clerk
     menyambungkannya ke user itu; dan
   - **Kas → Admin → Pengurus**: email yang sama + nama + peran. (Centang *Kirim
     undangan email* hanya kalau ingin Clerk mengirim tautan undangan sebagai
     ganti Create user.)
3. **Configure → API keys**: salin **Publishable key** (`pk_test_…`) dan **Secret
   key** (`sk_test_…`).
   - `pk_…` = publik → `VITE_CLERK_PUBLISHABLE_KEY` (build frontend).
   - `sk_…` = rahasia → `CLERK_SECRET_KEY` (backend).
4. Sementara memakai *development instance* (Google memakai kredensial bersama
   Clerk — cukup untuk tes). Saat domain siap: **Create production instance**,
   ikuti record DNS yang diminta Clerk, lalu ganti kedua kunci ke
   `pk_live_…`/`sk_live_…` (Tahap 6). Instance production **wajib kredensial
   Google sendiri**: di project Google Cloud akun RT (Tahap 3) buat *OAuth client
   ID → Web application* dengan *Authorized redirect URI* yang ditampilkan Clerk
   (**SSO connections → Google → Use custom credentials**), lalu tempel Client ID
   dan secret-nya di Clerk.

Login Clerk hanya memastikan *siapa* orangnya. *Haknya* ditentukan tabel
`pengurus` di database (admin / bendahara / komite) — email yang tidak terdaftar
di sana ditolak walau berhasil login Clerk.

## Tahap 3 — Akun Google RT (Drive + Sheet)

Akun biasa (gmail) tidak bisa memakai *service account* untuk menyimpan file
(service account tidak punya kuota Drive), jadi server bertindak **sebagai akun RT
lewat OAuth**.

1. Login ke akun Google RT. Buka **console.cloud.google.com** → buat project
   "Iuran Cypress".
2. **APIs & Services → Library**: aktifkan **Google Drive API** dan **Google
   Sheets API**.
3. **OAuth consent screen**: tipe *External*, isi nama app dan email akun RT.
   Tambahkan akun RT sebagai *test user*, lalu **Publish app → In production**.
   ⚠ Kalau dibiarkan *Testing*, refresh token **kedaluwarsa tiap 7 hari** dan
   upload bukti berhenti. App tidak perlu diverifikasi Google — hanya akun RT
   yang memakainya; layar "Google hasn't verified this app" cukup dilewati
   (*Advanced → Go to … (unsafe)*).
4. **Credentials → Create credentials → OAuth client ID → Desktop app.** Salin
   *Client ID* dan *Client secret* ke `backend/.env` (`GOOGLE_CLIENT_ID`,
   `GOOGLE_CLIENT_SECRET`).
5. Di komputer: `cd backend && go run ./cmd/google-login` → buka link yang
   dicetak di browser yang login **akun RT** → izinkan → terminal mencetak
   `GOOGLE_REFRESH_TOKEN=…`. Simpan ke `backend/.env` (dan nanti ke DigitalOcean).
6. **Drive akun RT**: buat folder "Bukti Iuran". ID-nya ada di URL
   `drive.google.com/drive/folders/<ID>` → `DRIVE_FOLDER_ID`. Semua bukti transfer
   dan nota pengeluaran baru masuk ke sini, privat.
7. **Bukti lama dari Google Form**: folder unggahan Form ada di Drive pemilik
   Form. Bagikan folder itu ke akun RT (**Viewer**), supaya Kas tetap bisa
   menampilkan bukti lama.
8. **Spreadsheet**: bagikan ke akun RT (**Viewer**). ID-nya dari URL
   `docs.google.com/spreadsheets/d/<ID>/edit` → `SHEET_ID`.

## Tahap 4 — Coba di komputer (opsional, tanpa Clerk & Google)

```sh
docker run -d --name hoacy-dev-db -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=hoacy \
  -p 127.0.0.1:55432:5432 postgres:17-alpine
cp backend/.env.example backend/.env       # isi BOOTSTRAP_ADMIN_EMAIL dengan email Anda
cd backend
go run ./cmd/sinkron -csv dev/sheet -terapkan   # data contoh, lewat jalur sinkron yang sama
go run ./cmd/server
# terminal lain, di root repo:
cp .env.example .env    # biarkan VITE_CLERK_PUBLISHABLE_KEY kosong untuk mode dev
npm install && npm run dev                 # http://localhost:5173
```

- Warga: Blok N7, No. 01, PIN `500001` (6 digit terakhir no. HP contoh).
- Kas: `http://localhost:5173/#/kas` → masukkan email `BOOTSTRAP_ADMIN_EMAIL`.
- Tes otomatis: `cd backend && go test ./...`.

## Tahap 5 — Deploy ke DigitalOcean

Ikuti `docs/deploy.md`. Ringkasnya: buat app dari `.do/app.yaml`, isi env var
rahasia (terenkripsi), deploy, buka `/api/sehat`.

## Tahap 6 — Pindah data dan rilis (cutover)

1. **Login pertama**: buka `/#/kas`, masuk dengan email `BOOTSTRAP_ADMIN_EMAIL`
   (otomatis jadi admin).
2. **Kas → Admin → Sinkron**, urut: `M-Blok` → `M-Rumah` → `M-RumahRiwayat` →
   `M-TarifVersi` → `L-Tunai` → `L-Transfer` → `L-Keputusan` → `L-Setoran` →
   `L-Pengeluaran` → setiap `M-Impor<tahun>`. Tiap tab: **Pratinjau** → periksa
   *Galat* (baris yang dilewati, mis. alamat salah ketik) → **Terapkan**.
   Bisa juga dari terminal: `go run ./cmd/sinkron -terapkan` (semua tab, urut).
   - PIN rumah **tidak** ikut dari Sheet: setiap rumah mulai dengan 6 digit
     terakhir no. HP-nya. Rumah tanpa no. HP mendapat PIN acak — reset di Admin →
     Rumah untuk melihat PIN sementara.
3. **Periksa**: Kas → Lihat semua kartu rumah. Tunggakan yang janggal biasanya
   berarti baris `M-Impor` terlewat atau baseline `M-RumahRiwayat` terlalu awal —
   perbaiki di Sheet, sinkron ulang tab itu.
4. **Admin → Sinkron → Opex & Saldo Awal → Muat ulang sekarang** (selanjutnya
   otomatis tiap hari).
5. **Pengurus**: untuk setiap orang, Clerk → Users → Create user (Gmail-nya),
   lalu Admin → Pengurus → email yang sama + nama + peran.
6. **Data sensitif di Sheet**: setelah sinkron `M-Rumah` beres, kosongkan kolom
   **telp** dan **pin** di Sheet — datanya sekarang hanya di database.
7. **Domain + Clerk production** (Tahap 2.4), lalu **cetak QR** (Kas → Cetak QR) —
   QR berisi alamat situs, jadi cetak setelah domain final.
8. Warga mulai memakai app 1 Januari 2027. Versi lama di Fly.io (`v0.1.0`) boleh
   dimatikan: `fly apps destroy iuran-cypress`.

Sinkron setelah rilis tetap bisa dipakai kapan saja, per kasus — mis. komite
menemukan pembayaran lama yang terlewat: tambahkan di `M-Impor<tahun>`, sinkron
tab itu.

---

## Perawatan rutin

- **Rumah baru**: Admin → Rumah → Tambah rumah, lalu isi riwayat luas/tipe
  (bulan mulai ditagih). Tanpa riwayat, rumah tampil "Tarif belum diatur" dan
  tidak bisa bayar.
- **Rumah membesar / kavling dibangun**: tambah baris riwayat mulai bulan itu.
- **Harga naik**: Admin → Tarif → *Rate card baru*, lengkap (termasuk aturan yang
  tidak berubah), berlaku mulai bulan kenaikan. ISLK rumah = harga daftar
  **dikurangi** Iuran RT.
- **Warga lupa PIN**: Admin → Rumah → Reset PIN (kembali ke 6 digit terakhir no. HP).
- **Merger / tidak tertagih lagi**: Admin → Rumah → Nonaktifkan (ketik alamatnya
  untuk konfirmasi). Alasannya dicatat di luar sistem.
- **Ada yang mencoba-coba PIN**: Admin → Keamanan. IP yang mencoba banyak rumah
  ditandai ⚠.
- **OPEX berubah**: edit `M-Opex` di Sheet — termuat otomatis besok, atau Admin →
  Sinkron → Muat ulang sekarang.
