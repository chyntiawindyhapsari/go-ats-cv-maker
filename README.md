# CV ATS Builder — Go

Aplikasi lokal pembuat CV ATS bahasa Indonesia/Inggris, dengan unduhan Word (.docx) dan PDF. Tidak memakai database.

## Kebutuhan
- Go 1.22+
- Internet saat pertama kali mengunduh dependency PDF

## Cara menjalankan di Windows
1. Ekstrak `ats-cv-go-project.zip`.
2. Buka CMD/PowerShell di folder `ats-cv-go`.
3. Jalankan `go mod tidy`.
4. Jalankan `go run .`.
5. Buka `http://localhost:8080`.

Untuk berhenti, tekan `Ctrl+C` pada terminal.

## Fitur
- Form data dan pratinjau langsung.
- Label bagian CV dalam bahasa Indonesia atau Inggris.
- Unduh DOCX dan PDF.
- Format sederhana satu kolom yang ramah ATS.
- Tidak menyimpan data ke database.

## Catatan
Contoh awal di formulir dibuat berdasarkan CV yang diberikan. Periksa kembali tanggal dan isi sebelum digunakan. Pilihan English mengubah label bagian; isi kolom perlu diterjemahkan sendiri untuk menghasilkan CV berbahasa Inggris penuh.
