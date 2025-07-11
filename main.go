package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/phpdave11/gofpdf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Struktur untuk Surat
type Surat struct {
	ID         uint      `gorm:"primaryKey"`
	Nomor      string    `gorm:"not null"`
	Nama       string    `gorm:"not null"`
	Tanggal    time.Time `gorm:"type:date;not null"`
	Jenis      string    `gorm:"not null"`
	Keterangan string
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// User struct untuk model data pengguna
type User struct {
	ID       uint   `gorm:"primaryKey"`
	Username string `gorm:"unique;not null"`
	Password string `gorm:"not null"`
}

// Data untuk dikirim ke template HTML
type HomeData struct {
	Surats      []Surat
	SearchQuery string
}

// Sambung ke databases
var database *gorm.DB

func connectDB() (*gorm.DB, error) {
	if database != nil {
		return database, nil
	}

	dsn := "root:@tcp(127.0.0.1:3306)/data_kampung?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}

	err = db.AutoMigrate(&Surat{}, &User{})
	if err != nil {
		return nil, fmt.Errorf("gagal auto migrate database: %w", err)
	}
	database = db
	return db, nil
}

// Untuk cek sudah login atau belum
func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("authenticated")
	return err == nil && cookie.Value == "true"
}

// homeHandler untuk halaman utama
func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Jika tidak sesuai kembali ke halaman login
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var surats []Surat
	query := db.Order("id asc")
	// pencarian sesuai nama atau nomor
	searchQuery := r.URL.Query().Get("q")
	if searchQuery != "" {
		query = query.Where("nomor LIKE ? OR nama LIKE ?",
			"%"+searchQuery+"%", "%"+searchQuery+"%")
	}

	result := query.Find(&surats)
	if result.Error != nil {
		http.Error(w, "Gagal mengambil data surat: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	data := HomeData{
		Surats:      surats,
		SearchQuery: searchQuery,
	}

	tmpl := template.Must(template.ParseFiles("template/index.html"))
	tmpl.Execute(w, data)
}

// Handler untuk tambah surat
func addSuratHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nomor := r.FormValue("Nomor")
	nama := r.FormValue("Nama")
	tanggalStr := r.FormValue("Tanggal")
	jenis := r.FormValue("Jenis")
	keterangan := r.FormValue("Keterangan")

	tanggal, err := time.Parse("2006-01-02", tanggalStr)
	if err != nil {
		http.Error(w, "Format tanggal tidak valid. Gunakan YYYY-MM-DD.", http.StatusBadRequest)
		log.Printf("Error parsing date: %v", err)
		return
	}

	validJenis := map[string]bool{
		"keterangan": true,
		"pengantar":  true,
	}
	if !validJenis[jenis] {
		http.Error(w, "Jenis surat tidak valid.", http.StatusBadRequest)
		return
	}

	surat := Surat{
		Nomor:      nomor,
		Nama:       nama,
		Tanggal:    tanggal,
		Jenis:      jenis,
		Keterangan: keterangan,
	}

	result := db.Create(&surat)
	if result.Error != nil {
		http.Error(w, "Gagal menyimpan data surat ke database: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Handler untuk mengedit surat
func editSuratHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idStr := r.FormValue("ID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "ID surat tidak valid", http.StatusBadRequest)
		return
	}

	// Ambil surat yang akan diedit terlebih dahulu untuk mendapatkan data yang ada
	var surat Surat
	result := db.First(&surat, id)
	if result.Error != nil {
		http.Error(w, "Surat tidak ditemukan: "+result.Error.Error(), http.StatusNotFound)
		return
	}

	// Ambil nilai dari form
	nomor := r.FormValue("Nomor")
	nama := r.FormValue("Nama")
	tanggalStr := r.FormValue("Tanggal")
	jenis := r.FormValue("Jenis")
	keterangan := r.FormValue("Keterangan")

	// Tanggal tidak wajib diganti
	if tanggalStr != "" {
		parsedTanggal, err := time.Parse("2006-01-02", tanggalStr)
		if err != nil {
			http.Error(w, "Format tanggal tidak valid. Gunakan YYYY-MM-DD.", http.StatusBadRequest)
			log.Printf("Error parsing date for edit: %v", err)
			return
		}
		surat.Tanggal = parsedTanggal // Perbarui tanggal jika ada input baru
	}
	// Jika tanggal surat kosong, Tanggal akan tetap menggunakan nilai lama dari database.

	// Validasi jenis surat
	validJenis := map[string]bool{
		"keterangan": true,
		"pengantar":  true,
	}
	if !validJenis[jenis] {
		http.Error(w, "Jenis surat tidak valid.", http.StatusBadRequest)
		return
	}

	// Update surat
	surat.Nomor = nomor
	surat.Nama = nama
	surat.Jenis = jenis
	surat.Keterangan = keterangan

	result = db.Save(&surat)
	if result.Error != nil {
		http.Error(w, "Gagal mengupdate data surat: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Handler untuk hapus
func deleteSuratHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "ID surat tidak valid", http.StatusBadRequest)
		return
	}

	var surat Surat
	result := db.First(&surat, id)
	if result.Error != nil {
		http.Error(w, "Surat tidak ditemukan: "+result.Error.Error(), http.StatusNotFound)
		return
	}

	result = db.Delete(&surat, id)
	if result.Error != nil {
		http.Error(w, "Gagal menghapus surat dari database: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Handler untuk export PDF
func ExportPDFHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var surats []Surat
	result := db.Find(&surats)
	if result.Error != nil {
		http.Error(w, "Gagal mengambil data surat dari database: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Daftar Arsip Surat RT", "0", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(30, 10, "Nomor", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 10, "Nama", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 10, "Tanggal", "1", 0, "C", false, 0, "")
	pdf.CellFormat(35, 10, "Jenis", "1", 0, "C", false, 0, "")
	pdf.CellFormat(60, 10, "Keterangan", "1", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)

	for _, surat := range surats {
		nomor := surat.Nomor
		if len(nomor) > 20 {
			nomor = nomor[:20] + "..."
		}
		nama := surat.Nama
		if len(nama) > 25 {
			nama = nama[:25] + "..."
		}
		jenis := surat.Jenis
		if len(jenis) > 20 {
			jenis = jenis[:20] + "..."
		}
		keterangan := surat.Keterangan
		if len(keterangan) > 40 {
			keterangan = keterangan[:40] + "..."
		}

		pdf.CellFormat(30, 10, nomor, "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 10, nama, "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 10, surat.Tanggal.Format("02-01-2006"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(35, 10, jenis, "1", 0, "C", false, 0, "")
		pdf.CellFormat(60, 10, keterangan, "1", 1, "L", false, 0, "")
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="daftar_surat.pdf"`)
	if err := pdf.Output(w); err != nil {
		log.Printf("Gagal menulis PDF ke response: %v", err)
		return
	}
}

// Handler untuk login
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Jika sudah login, redirect ke halaman utama
	if isAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == http.MethodGet {
		tmpl := template.Must(template.ParseFiles("template/login.html"))
		tmpl.Execute(w, nil)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		db, err := connectDB()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var user User
		result := db.Where("username = ?", username).First(&user)

		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Username atau password salah!", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Terjadi kesalahan server: "+result.Error.Error(), http.StatusInternalServerError)
			return
		}

		// jika password salah
		if user.Password != password {
			http.Error(w, "Username atau password salah!", http.StatusUnauthorized)
			return
		}

		// Login berhasil
		http.SetCookie(w, &http.Cookie{
			Name:    "authenticated",
			Value:   "true",
			Path:    "/",
			Expires: time.Now().Add(24 * time.Hour),
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
}

// Handler untuk logout
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "authenticated",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func main() {
	_, err := connectDB()
	if err != nil {
		log.Fatalf("Gagal terhubung ke database: %v", err)
	}

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/surats", homeHandler)
	http.HandleFunc("/tambah", addSuratHandler)
	http.HandleFunc("/edit", editSuratHandler)
	http.HandleFunc("/hapus", deleteSuratHandler)
	http.HandleFunc("/export-pdf", ExportPDFHandler)

	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)

	log.Println("Server Arsip Surat RT berjalan di http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
