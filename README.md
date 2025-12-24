Hardix Audio Container (HDX) - Specification & Toolchain
Dokumentasi ini mendefinisikan standar kontainer audio aman HDX (Hardix) versi 2.0.0. Format ini dirancang untuk mendistribusikan audio terenkripsi dengan metadata dinamis menggunakan pendekatan Hybrid TLV-JSON.

1. HDX-Struct (Container Specification)
Format .hdxv menggunakan arsitektur biner kustom yang mengoptimalkan keamanan per-frame dan kecepatan inspeksi metadata melalui metode Reverse-Search.

Teknologi Utama
Audio Codec: Opus Interactive Audio Codec (Standard: 48kHz, Stereo).

Encryption: AES-GCM (Galois/Counter Mode) dengan 12-byte random nonce untuk setiap frame audio.

Key Derivation: PBKDF2 dengan SHA-256 (4096 iterasi) untuk menghasilkan 32-byte kunci enkripsi.

Security Engine: Sistem Key Locker (_keys.dat) untuk mengamankan password album menggunakan Master Brute-Force Key gabungan.

Struktur Biner File (.hdxv)
Urutan penulisan data dilakukan secara sekuensial sebagai berikut:

Header Utama: Magic Number HDXV02 (6 Bytes).

Initial Metadata (TLV Block): Serangkaian tag identitas statis menggunakan format Tag (4 bytes) | Length (uint32) | Value.

ALBM: Nama Album.

GENR: Artist atau Genre.

PUBL: Publisher.

CPRT: Copyright.

VOLI: Volume Info.

RLSD: Release Date.

CRDT: Created Date.

Audio Data Area (ADAT):

Marker AUDI (4 bytes).

Total ADAT Size (4 bytes uint32).

Payload: Deretan frame Opus terenkripsi AES-GCM, tiap frame diawali 2 byte panjang data (uint16).

Artwork (ARTW): Tag ARTW (4 bytes) + Length (4 bytes) + Data Biner Gambar (JPEG/PNG).

JSON Footer (JSFD): Marker JSFD (4 bytes) + Length (4 bytes) + Seluruh isi VolumeStructure dalam format JSON sebagai penutup file.

2. HDX-Volmaker (The Forger)
Alat baris perintah (CLI) untuk merakit file WAV dan metadata menjadi kontainer .hdxv.

Alur Kerja (Code Flow)
Interview Phase: Mengumpulkan path JSON, direktori output, dan password melalui terminal interaktif.

Key Preparation: Menghasilkan audioKey secara dinamis menggunakan PBKDF2.

Streaming Audio Loop:

Membuka WAV dan melakukan encoding ke Opus secara sekuensial.

Enkripsi setiap frame secara real-time menggunakan AES-GCM.

Menghitung durasi dan fingerprint track secara otomatis.

Artwork Injection: Membaca file gambar dari ArtworkPath dan menyuntikkannya ke dalam kontainer menggunakan tag biner ARTW.

Final Sealing: Memperbarui ukuran ADAT di header, menulis dump JSON ke marker JSFD, dan membuat file kunci keamanan _keys.dat.

Fitur Unggulan
Atomic Forging: Pipeline encoding dan enkripsi berjalan simultan tanpa file sementara.

Memory Efficiency: Menggunakan rutin runtime.GC() dan debug.FreeOSMemory() untuk menjaga penggunaan RAM saat memproses album besar.

3. HDX-Meta (The Inspector)
Alat audit untuk melihat informasi detail file tanpa perlu mendekripsi data audio.

Alur Analisis
Tail-Jump: Melompat ke 10MB terakhir file untuk mencari metadata.

Marker Discovery: Mencari posisi string JSFD dan ARTW menggunakan strings.LastIndex (pencarian mundur).

Metadata Extraction: Mengambil raw JSON dan melakukan unmarshal ke struct VolumeStructure untuk menampilkan detail track dan durasi.

Artwork Dump (-artdump): Menganalisis tag ARTW, mendeteksi tipe file via magic bytes, dan menghitung ukuran file secara human-friendly (Kb/Mb).

Fitur Utama
Lightweight Inspection: Membaca metadata file berukuran GB dalam milidetik.

Artwork Validator: Mendeteksi integritas dan tipe biner artwork yang tertanam.