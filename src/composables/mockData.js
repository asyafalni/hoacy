// Local-dev fixture only. useSheet.js falls back to this when VITE_SHEET_ID is
// unset, so the app is clickable without a real Google Sheet. Shape matches
// exactly what parse() would produce from the API tab's gviz response — see
// docs/sheets-schema.md §6 for the real column layout (A/B key-value interleaved
// with D..Z per-house columns on the same sheet rows).
const S = { L: 'Lunas', S: 'Sebagian', P: 'Pending', B: 'Belum', '-': '-' };

const houses = [
  { alamat: 'N7-01', nama: 'Budi Santoso',  telp: '6281234500001', luas: 130, tipe: 'rumah',
    tarif: 300000, tunggakan: 300000, cluster: 'N', blok: 7, rumah: '01', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.B,S['-'],S['-'],S['-']] },
  { alamat: 'N7-03', nama: 'Siti Aminah',    telp: '6281234500003', luas: 180, tipe: 'rumah',
    tarif: 360000, tunggakan: 540000, cluster: 'N', blok: 7, rumah: '03', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.S,S.P,S.B,S['-'],S['-'],S['-']] },
  { alamat: 'N7-05', nama: 'Dewi Lestari',   telp: '6281234500005', luas: 230, tipe: 'rumah',
    tarif: 360000, tunggakan: 0, cluster: 'N', blok: 7, rumah: '05', mukaTahunDepan: '1,2',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L] },
  { alamat: 'N8-02', nama: 'Agus Wibowo',    telp: '6281234500102', luas: 400, tipe: 'rumah',
    tarif: 450000, tunggakan: 4050000, cluster: 'N', blok: 8, rumah: '02', mukaTahunDepan: '',
    status: [S.B,S.B,S.B,S.B,S.B,S.B,S.B,S.B,S.B,S['-'],S['-'],S['-']] },
  { alamat: 'N8-05', nama: 'Rina Kusuma',    telp: '6281234500105', luas: 50,  tipe: 'kavling',
    tarif: 70000,  tunggakan: 35000, cluster: 'N', blok: 8, rumah: '05', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.S,S['-'],S['-'],S['-']] },
  { alamat: 'N7-09', nama: 'Hendra Saputra', telp: '6281234500009', luas: 260, tipe: 'rumah',
    tarif: 425000, tunggakan: 425000, cluster: 'N', blok: 7, rumah: '09', mukaTahunDepan: '',
    status: [S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.L,S.B,S['-'],S['-'],S['-']] },
];

const meta = [
  ['kas_tunai', 1750000],
  ['rekening', 6250000],
  ['tunggakan_total', houses.reduce((sum, h) => sum + h.tunggakan, 0)],
  ['lunas_bulan_ini', houses.filter((h) => h.status[8] === S.L).length],
  ['jumlah_rumah', houses.length],
  ['updated', '2026-09-22 09:00'],
  ['tahun_aktif', 2026],
];

const houseRow = (h) => [
  h.alamat, h.nama, h.telp, h.luas, h.tipe, h.tarif, h.tunggakan,
  ...h.status, h.cluster, h.blok, h.rumah, h.mukaTahunDepan,
];

export const MOCK_ROWS = meta.map(([k, v], i) =>
  [k, v, null, ...(houses[i] ? houseRow(houses[i]) : Array(23).fill(null))]);
