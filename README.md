# Problem

Belum ada Open Source Scheduler buatan indonesia yang sangat customable

Existing : 

**Open Cluster** Management (OCM), Karmada di khususkan untuk kubernetes, belum untuk solusi umum

Volcano di khususkan untuk Ai

**HashiCorp Nomad:** yang kita bangun semacam ini , tapi kita terbuka tidak terbatas jenis engine  container terntentu

# Approach

Pattern Control Plane x Data Plane

# Scope [Demo Version]

- Simple Scheduler
- Provisioning with incus

# Control Plane x Data Plane

## Analogi

Ini adalah salah satu konsep terpenting dalam jaringan modern. Cara termudah untuk memikirkannya adalah menggunakan **analogi Ketua Partai Politik dan Anggotanya**.

- **Control Plane (Si "Ketua Partai" / Petinggi Partai):** Ini adalah pusat strategi partai.
    - Tugasnya adalah "berpikir", "merumuskan strategi", dan "memutuskan arahan".
    - Ia mengumpulkan informasi (data survei, masukan dari daerah, isu politik, tujuan partai).
    - Ia menghitung *strategi terbaik* untuk menang (misalnya, menentukan isu kampanye, daerah prioritas).
    - Ia **tidak** benar-benar turun ke setiap TPS atau mengetuk setiap pintu rumah pemilih.
- **Data Plane (Si "Anggota / Kader Partai"):** Ini adalah para kader yang bekerja di lapangan.
    - Tugasnya adalah "bertindak" dan "melaksanakan" arahan.
    - Ia mengambil instruksi spesifik dari pusat (misalnya, "Sebarkan poster A," "Fokus pada isu B," "Kunjungi wilayah C").
    - Ia *secara fisik menemui pemilih* (data/paket) dan menyampaikan pesan partai untuk mencapai tujuan.
    - Ia bekerja serentak, cepat, dan masif di banyak tempat, tetapi hanya melakukan apa yang diinstruksikan oleh si "ketua".

### Mengapa Memisahkan Keduanya?

- **Skalabilitas:** Anda bisa memiliki satu "otak" / "Pusat" (Control Plane) yang kuat yang mengelola *banyak* "kader di lapangan" (Data Plane) yang sederhana. Ini adalah ide inti dari **Software-Defined Networking (SDN)** dan juga Kubernetes.
- **Ketahanan (Resilience):** Jika "Pusat" (Control Plane Kubernetes) *crash*, para "Kader" (Data Plane ) akan terus menjalankan aplikasi (*pods*) yang sudah ada berdasarkan arahan terakhir. Aplikasi Anda tetap berjalan.
- **Fleksibilitas:** Anda dapat memperbarui "strategi partai" (misalnya, meng-upgrade Kubernetes Control Plane) tanpa harus menghentikan dan melatih ulang semua kader (me-reboot semua *worker node*) secara bersamaan.
- **Manajemen Lingkungan Dinamis:** Seperti dalam contoh Kubernetes, "kader" (Data Plane, atau *pods* dan *nodes*) bisa muncul dan hilang dengan cepat. dan bisa jadi kader (Node) bisa berubah alamat (IP). "Pusat" (Control Plane) tidak perlu melacak setiap perubahan secara manual. Sebaliknya, setiap "Kader" baru (Node) secara proaktif mendaftarkan dirinya ke "Pusat" , sehingga klaster dapat mengelola dirinya sendiri secara dinamis.
- Control Plane (pusat), tidak perlu hit ke semua node (data plane), untuk memastikan kesehatan
    
    Mengunakan konsep heartbeat , data plane yang akan mengabarkan kesehatan ke pusat

<img width="803" height="345" alt="image3" src="https://github.com/user-attachments/assets/3188ba14-b49a-49d4-a1cf-a267e7ac0f2c" />
<img width="771" height="231" alt="image2" src="https://github.com/user-attachments/assets/00d6432d-9e93-470d-bf25-f45a85332734" />
<img width="999" height="521" alt="image" src="https://github.com/user-attachments/assets/d26cad68-f52f-4e6d-8bb8-907a48d2f963" />
