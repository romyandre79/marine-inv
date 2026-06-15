# Inventory Management System

Inventory Management System adalah sub-sistem yang menangani logistik kapal, manajemen suku cadang (spare parts), bahan makanan/perbekalan (provisions), dan pengadaan barang (procurement/purchasing).

## 🌟 Fitur Utama
* **Spare Parts Management**: Pencatatan suku cadang mesin, nomor seri, lokasi penyimpanan (gudang darat maupun di atas kapal).
* **Stock & Reorder Point Alert**: Notifikasi otomatis ketika stok barang tertentu di kapal berada di bawah batas minimum agar segera dilakukan pengadaan.
* **Purchase Request & Order**: Siklus pengajuan kebutuhan barang dari kapal (Purchase Request) hingga persetujuan dan pemesanan ke vendor (Purchase Order).

## 🛠️ Tech Stack
* **Frontend**: Nuxt 4 (Vue 3), Tailwind CSS
* **Backend**: Golang (REST API)
* **Database**: PostgreSQL (Database `dev-inventory`)

## 🚀 Port & Domain
* **Development Frontend Port**: `3013`
* **Development Backend Port**: `3014`

## 📂 Struktur Direktori
* `/frontend`: Dashboard Nuxt 4 untuk manajemen stok dan logistik.
* `/backend`: REST API Golang untuk manajemen inventory dan alur purchasing.

## 📦 Cara Memulai

### Backend
```bash
cd backend
# Setup .env
go run cmd/server/main.go
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```
