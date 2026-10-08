/* eslint-disable no-undef */
// Google Apps Script wrapper — the only part of the server that touches Google
// services. Appended after the bundled src/lib + src/server/core.js code by
// scripts/build-apps-script.mjs; never imported by the app itself.
//
// Deploy: Extensions → Apps Script in the Iuran_ClusterN spreadsheet, paste
// apps-script/Code.gs, set Script Properties POS_PIN / KAS_PIN, then
// Deploy → New deployment → Web app (Execute as: Me, Who has access: Anyone).
// docs/setup.md, Tahap 6.

function bacaTab(nama) {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheet = ss.getSheetByName(nama);
  if (!sheet) throw new Error('Tab tidak ditemukan: ' + nama);
  var tz = ss.getSpreadsheetTimeZone();
  var rows = sheet.getDataRange().getValues().slice(1).map(function (r) {
    return r.map(function (c) {
      if (c === '') return null;
      if (c instanceof Date) {
        var jam = Utilities.formatDate(c, tz, 'HH:mm:ss');
        return Utilities.formatDate(c, tz, jam === '00:00:00' ? 'yyyy-MM-dd' : 'yyyy-MM-dd HH:mm:ss');
      }
      return c;
    });
  });
  while (rows.length && rows[rows.length - 1].every(function (c) { return c === null; })) rows.pop();
  return rows;
}

function buatKonteks() {
  var cache = CacheService.getScriptCache();
  var props = PropertiesService.getScriptProperties();
  return {
    tab: bacaTab,
    prop: function (k) { return props.getProperty(k); },
    cache: {
      get: function (k) { return cache.get(k); },
      put: function (k, v, detik) { cache.put(k, v, detik); },
      remove: function (k) { cache.remove(k); },
    },
    hari: function () {
      var tz = SpreadsheetApp.getActiveSpreadsheet().getSpreadsheetTimeZone();
      var p = Utilities.formatDate(new Date(), tz, 'yyyy-M').split('-');
      return { tahun: Number(p[0]), bulan: Number(p[1]) };
    },
  };
}

function jawab(obj) {
  return ContentService.createTextOutput(JSON.stringify(obj)).setMimeType(ContentService.MimeType.JSON);
}

// The app POSTs JSON as text/plain — a "simple" request, so the browser skips
// the CORS preflight that Apps Script web apps can't answer.
function doPost(e) {
  var req;
  try { req = JSON.parse(e.postData.contents); } catch (err) { return jawab({ ok: false, kode: 'request' }); }
  return jawab(handle(req, buatKonteks()));
}

// Open the /exec URL in a browser to check the deployment is live.
function doGet() {
  return jawab({ ok: true, pesan: 'Iuran API aktif' });
}
