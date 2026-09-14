# AGENT INSTRUCTION: Go Native Backend Architecture for AI Diagram Generator

Dokumen ini berisi spesifikasi arsitektur, aturan koding (coding standards), struktur direktori, dan langkah eksekusi (implementation checklist) untuk membangun backend API generator diagram interaktif.

---

## 1. PROJECT SPECIFICATION & TECH STACK

- **Language:** Go (Golang) versi 1.22+
- **HTTP Server & Routing:** Standard Library `net/http` murni (tanpa framework Gin/Fiber/Echo).
- **Database & Driver:** PostgreSQL menggunakan driver native `github.com/lib/pq` atau `github.com/jackc/pgx/v5/stdlib`.
- **Environment Management:** `github.com/joho/godotenv` untuk memuat variabel `.env`.
- **LLM Integration:** Direct HTTPS client (`net/http`) ke Anthropic Messages API (`claude-3-5-sonnet-20241022`).
- **Entry Point:** Root `main.go`. Seluruh logika aplikasi terisolasi di dalam direktori `src/`.

---

## 2. DIRECTORY STRUCTURE RULES

Wajib mengikuti struktur arsitektur berlapis (Clean/Layered Pattern) berikut:

```text
.
├── .env.example
├── .env
├── .gitignore
├── agent.md
├── go.mod
├── go.sum
├── main.go
└── src/
    ├── config/          # Inisialisasi Database, Environment, dan Client pihak ketiga
    ├── controllers/     # HTTP Request parsing, validasi DTO, dan JSON response
    ├── dtos/            # Data Transfer Objects (Payload Request & Response JSON)
    ├── entities/        # Representasi tabel PostgreSQL & Domain Structs
    ├── middlewares/     # CORS native, Logging, Recovery, Authentication (jika ada)
    ├── models/          # Data Access Layer / Repositories (Raw SQL queries ke PostgreSQL)
    ├── routes/          # Mapping HTTP Route (Go ServeMux) ke Controller Handlers
    ├── services/        # Business Logic & Integrasi Claude API Prompting
    └── utils/           # Helper JSON response, error wrapper, sanitasi string
```
