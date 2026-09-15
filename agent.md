# AGENT ARCHITECTURE & DEVELOPMENT MANUAL
## Visual Workspace: AI Diagramming & Modular AI UI Design Canvas

Dokumen ini adalah panduan arsitektur komprehensif, standar modularitas, skema database, spesifikasi API, dan petunjuk operasional pengembangan untuk platform workspace visual cerdas yang menggabungkan:
1. **AI Diagram & Flowchart Engine**: Visualisasi arsitektur cloud, flowchart logika, ERD relasional, UML class, state machine, pipeline data, dan BPMN swimlane.
2. **Modular AI UI Design Canvas**: Generator visual UI sections responsif (Web 1024px, Mobile Smartphone 375px, Desktop Dashboard 1100px) terinspirasi oleh *desainpakeai.com*, lengkap dengan pratinjau visual dan ekspor kode (Tailwind CSS & Vue 3).

---

## 1. CORE PRINCIPLES & STRICT MODULARITY RULE

Untuk menjaga maintainability sistem dalam jangka panjang, aplikasi memberlakukan **Separation of Concerns (SoC)** yang ketat:

> **Aturan Isolasi Modul (Strict Modularity Boundary):**
> 1. Modul **Diagram** dan modul **UI Design** TIDAK BOLEH disatukan atau saling mengunci (*tight coupling*).
> 2. Di backend:
>    - Logika diagram dikelola oleh `project_service.go`, `project_controller.go`, dan `dtos/project_dto.go`.
>    - Logika UI design dikelola secara independen oleh `ui_design_service.go`, `ui_design_controller.go`, `dtos/ui_design_dto.go`, dan `models/ui_template_model.go`.
>    - Masing-masing modul memiliki endpoint terpisah (`/api/projects/*` vs `/api/ui-design/*`).
> 3. Di frontend:
>    - Node diagram flowchart/ERD berada di `src/components/nodes/`.
>    - Node UI frame & sub-section berada di `src/components/ui-design/` dan `src/components/ui-design/sections/`.
>    - Kanvas Vue Flow bertindak sebagai *unifying host canvas* yang mengenali kedua tipe node secara dinamis via slot registry (`#node-ui_frame`, `#node-swimlane`, `#node-database`, dsb.).

---

## 2. TECH STACK & SYSTEM ARCHITECTURE

### Backend (Go Native Micro-service)
- **Language**: Go (Golang) 1.22+
- **HTTP Server & Routing**: Standard Library `net/http` native dengan routing pattern Go 1.22 (`POST /api/projects/{id}/chat`).
- **Database**: PostgreSQL dengan driver native `github.com/lib/pq`.
- **Environment**: `github.com/joho/godotenv` untuk konfigurasi `.env`.
- **LLM Engine**: Multi-provider direct HTTPS client (Anthropic Messages API / OpenRouter / OpenAI compatible fallback).

### Frontend (Modern Vue 3 SPA)
- **Framework**: Vue 3 (Composition API `<script setup>`) + Vite
- **Canvas Engine**: `@vue-flow/core`, `@vue-flow/background`, `@vue-flow/controls`, `@vue-flow/minimap`
- **Layout Graph Engine**: `dagre` (Dagre Graphlib) dengan custom anti-collision swimlane & UI frame offset engine
- **Styling**: Tailwind CSS + Modern Glassmorphism & Micro-animations
- **Iconography**: `lucide-vue-next`
- **Export Capabilities**: SVG, PNG 2x HD via `html-to-image`, Notion/Mermaid code, Raw JSON, dan Vue 3 / Tailwind Component Code.

---

## 3. REPOSITORY DIRECTORY STRUCTURE

```text
/var/www/project-diagram/
├── backend-diagram/               # Go Backend Microservice
│   ├── .env                       # Database & LLM API Keys
│   ├── agent.md                   # System Architecture Manual (Dokumen ini)
│   ├── go.mod / go.sum
│   ├── main.go                    # Dependency Injection & Server Entry Point
│   └── src/
│       ├── config/                # Database & Environment Loader
│       ├── controllers/
│       │   ├── project_controller.go     # Controller Modul Diagram
│       │   ├── template_controller.go    # Controller Template Diagram
│       │   └── ui_design_controller.go   # Controller Modul UI Design Canvas (ISOLATED)
│       ├── dtos/
│       │   ├── project_dto.go            # DTO Modul Diagram
│       │   ├── template_dto.go           # DTO Template Diagram
│       │   └── ui_design_dto.go          # DTO Modul UI Design (ISOLATED)
│       ├── entities/
│       │   ├── project.go                # Entity Projects (ProjectMode, Metadata)
│       │   ├── project_version.go        # Entity Versi Riwayat
│       │   ├── message.go                # Entity Chat Message
│       │   ├── diagram_template.go       # Entity Template Diagram
│       │   └── ui_template.go            # Entity Template UI Design (ISOLATED)
│       ├── models/
│       │   ├── project_model.go          # Data Access Projects (SQL Native)
│       │   ├── project_version_model.go  # Data Access Riwayat Versi
│       │   ├── message_model.go          # Data Access Chat History
│       │   ├── template_model.go         # Data Access Template Diagram
│       │   └── ui_template_model.go      # Data Access Template UI Design (ISOLATED)
│       ├── routes/
│       │   └── api.go                    # Route Registry ServeMux Go 1.22
│       ├── services/
│       │   ├── ai_service.go             # Generic LLM Client & JSON Sanitizer
│       │   ├── project_service.go        # Business Logic Diagram
│       │   └── ui_design_service.go      # Business Logic AI UI Design Canvas (ISOLATED)
│       └── utils/
│           └── response.go               # Standardized JSON Response Formatter
│
├── frontend-diagram/              # Vue 3 Client Application
│   ├── package.json
│   ├── vite.config.js
│   └── src/
│       ├── App.vue                # Root Workspace Layout & Mode Switcher
│       ├── assets/
│       ├── composables/
│       │   ├── useDiagramApi.js   # HTTP Client API Services
│       │   ├── useGraphLayout.js  # Dagre Layout & UI Frame Offset Engine
│       │   └── useCanvasExport.js # PNG, SVG, Mermaid, JSON Export
│       ├── stores/
│       │   └── diagramStore.js    # Pinia/Vue State Management
│       └── components/
│           ├── canvas/
│           │   └── DiagramCanvas.vue      # Vue Flow Host Canvas with Dynamic Slots
│           ├── layout/
│           │   ├── ChatPanel.vue          # Context-Aware AI Chat Inspector
│           │   └── SidebarPanel.vue       # Project Browser with Type Badges
│           ├── nodes/                     # Node Khusus Modul Diagram
│           │   ├── StartEndNode.vue
│           │   ├── ProcessNode.vue
│           │   ├── DecisionNode.vue
│           │   ├── DatabaseNode.vue
│           │   └── SwimlaneLaneNode.vue
│           ├── ui-design/                 # Modul UI Design Canvas (ISOLATED)
│           │   ├── UiFrameNode.vue        # Device Frame Node (Web, Mobile, Desktop)
│           │   └── sections/              # Komponen Section UI Dinamis
│           │       ├── UiNavbarSection.vue
│           │       ├── UiHeroSection.vue
│           │       ├── UiKpiSection.vue
│           │       ├── UiTableSection.vue
│           │       ├── UiMobileStatusSection.vue
│           │       ├── UiBalanceSection.vue
│           │       ├── UiAssetListSection.vue
│           │       ├── UiMobileNavSection.vue
│           │       ├── UiAnnouncementSection.vue
│           │       ├── UiFormSection.vue       # Auth & Custom Input Forms
│           │       ├── UiFeatureGridSection.vue# Core Architectural & Business Benefits
│           │       ├── UiProductGridSection.vue# E-Commerce Catalog & Storefront Grid
│           │       └── UiPricingSection.vue    # SaaS Pricing & Subscription Tiers
│           └── ui/
│               ├── NewProjectModal.vue    # Creation Modal (Diagram, UI Design, Templates)
│               ├── ExportDropdown.vue     # Dropdown Export (Image, JSON, Tailwind/Vue Code)
│               └── VersionHistoryModal.vue# Riwayat Snapshot Versi & Rollback
```

---

## 4. DATABASE SCHEMA & PERSISTENCE

Semua data tersimpan secara terstruktur di PostgreSQL (`diagram_db`).

```sql
-- 1. Tabel Projects (Mendukung mode diagram & ui_design, serta pinned project)
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    diagram_type VARCHAR(100) NOT NULL DEFAULT 'flowchart',
    project_mode VARCHAR(50) NOT NULL DEFAULT 'diagram', -- 'diagram' | 'ui_design'
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB DEFAULT '{}'::jsonb,
    current_nodes JSONB NOT NULL DEFAULT '[]'::jsonb,
    current_edges JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indeks performa untuk pinned project & sorting
CREATE INDEX IF NOT EXISTS idx_projects_pinned_updated ON projects(is_pinned DESC, updated_at DESC);

-- 2. Tabel Project Versions (Snapshot untuk riwayat & rollback)
CREATE TABLE IF NOT EXISTS project_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    summary TEXT,
    nodes JSONB NOT NULL DEFAULT '[]'::jsonb,
    edges JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. Tabel Messages (Percakapan iteratif AI)
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL, -- 'user' | 'assistant'
    content TEXT NOT NULL,
    targeted_node_ids JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 4. Tabel Diagram Templates (Template bawaan flowchart, BPMN, ERD)
CREATE TABLE IF NOT EXISTS diagram_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    diagram_type VARCHAR(100) NOT NULL,
    category VARCHAR(100) NOT NULL,
    nodes JSONB NOT NULL,
    edges JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 5. Tabel UI Templates (Template bawaan Web, Mobile, Desktop UI Frames)
CREATE TABLE IF NOT EXISTS ui_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,
    device VARCHAR(50) NOT NULL, -- 'web' | 'mobile' | 'desktop'
    description TEXT,
    theme JSONB NOT NULL DEFAULT '{}'::jsonb,
    sections JSONB NOT NULL DEFAULT '[]'::jsonb,
    code_export JSONB NOT NULL DEFAULT '{}'::jsonb,
    width INT NOT NULL DEFAULT 1024,
    height INT NOT NULL DEFAULT 720,
    is_featured BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

---

## 5. API SPECIFICATION & ROUTING

### A. Modul Diagram (`/api/projects/*` & `/api/templates/*`)
- `GET /api/projects`: Mengambil daftar seluruh project (ringkasan ID, judul, tipe, tanggal).
- `POST /api/projects`: Membuat project diagram baru dari prompt AI atau template diagram.
- `GET /api/projects/{id}`: Mengambil detail lengkap project (nodes, edges, chat history).
- `POST /api/projects/{id}/chat`: Mengirim prompt iterasi AI untuk diagram (dengan targeted nodes).
- `GET /api/projects/{id}/versions`: Mengambil riwayat versi diagram.
- `POST /api/projects/{id}/rollback/{version_id}`: Melakukan rollback canvas ke versi tertentu.
- `GET /api/templates`: Mengambil daftar template diagram (Flowchart, BPMN, ERD, Cloud Arch).

### B. Modul UI Design Canvas (`/api/ui-design/*`)
- `GET /api/ui-design/templates`: Mengambil galeri template UI design (Web, Mobile, Desktop).
- `POST /api/ui-design/generate`: Membuat project UI design baru.
  - Payload:
    ```json
    {
      "prompt": "SaaS Cloud Analytics Dashboard dengan KPI revenue",
      "device": "web",            // "web" | "mobile" | "desktop" | "multi"
      "theme": "dark",            // "dark" | "violet" | "light"
      "foundation": "ramp",       // "ramp" | "calcom" | "raycast" | "railway" | "attio" | "mintlify"
      "template_id": null         // opsional UUID jika instant clone
    }
    ```
- `POST /api/ui-design/projects/{id}/chat`: Mengirim instruksi revisi UI ke AI (dengan konteks fondasi dan targeted frames).

---

## 6. UI DESIGN FRAME & SECTION SYSTEM SPECIFICATION

Modul UI Design merepresentasikan antarmuka pengguna nyata di dalam kanvas visual menyerupai *desainpakeai.com*.

### A. Frame Shells
1. **Web & Desktop Frame (`device: 'web'` atau `'desktop'`)**:
   - Header Bar Browser Chrome dengan *macOS traffic lights* (Merah, Kuning, Hijau).
   - Mock SSL Address Bar (`https://app.[nama-produk].io`).
   - Resolusi Badge (`1024 × 720` atau `1100 × 740`).
   - Anti-Slop Quality Check Badge dengan popover audit skor 100%.
   - Canvas Review Feedback Pins (`ReviewCommentPin.vue`) dengan status open/resolved dan 1-click kirim prompt ke AI.
   - Switcher Mode: `[ 👁️ Preview ]` (tampilan visual render) vs `[ 💻 Code ]` (syntax viewer).
   - Tombol Instant Clipboard Copy untuk kode komponen.
2. **Mobile Smartphone Frame (`device: 'mobile'`)**:
   - Smartphone shell rounded bezel 48px dengan frame tebal dan ring glow.
   - Notch / Dynamic Island di bagian atas.
   - Status bar iOS/Android (Waktu `09:41`, Wifi, Baterai).
   - Home Indicator Bar di bagian bawah.

### B. Dual-Engine Rendering (Freedom of Expression)
1. **Bespoke Tailwind HTML (`raw_html`)**: Untuk prompt unik (misal: Gym workout tracker, audio player, calendar scheduler, kanban board, filter properti, resep makanan), sistem merender Tailwind HTML murni yang dihasilkan AI langsung di dalam frame mockup sehingga 100% selaras dengan imajinasi user.
2. **Modular Sections (`sections`)**: Untuk layout standar seperti auth, landing page, atau SaaS analytics dashboard, sistem merender komponen section terstruktur (`UiNavbarSection`, `UiHeroSection`, `UiKpiGridSection`, `UiDataTableSection`, dsb.).

### C. Design Foundations & Anti-Slop Principles
Tersedia 6 fondasi desain:
- `ramp`: Ramp Clean (Neutral gray #f8fafc canvas, white surfaces, subtle borders, high contrast, 8px radius)
- `calcom`: Cal.com Calm (Minimal calm gray, dark slate accent, generous breathing room)
- `raycast`: Raycast Keyboard (Deep dark #0c0d0e, electric coral/violet accents, monospace keyboard shortcuts)
- `railway`: Railway Terminal (Dark violet infrastructure, purple accents, telemetry log styling)
- `attio`: Attio Fluid (Ultra-clean modern CRM, dense data tables, blue accents, 6px radius)
- `mintlify`: Mintlify Knowledge (Modern developer documentation, dark surfaces, emerald accents)

Anti-AI Slop Rules:
1. NO glowing purple orbs, NO arbitrary floating colored blobs, NO ornamental background radial gradients.
2. NO excessive blurry glassmorphism.
3. Clean high-contrast typography dengan hierarki tegas.
4. Data nyata spesifik domain (NO Lorem Ipsum, NO Sample Title).

---

## 8. WORKFLOW PENAMBAHAN FITUR BARU

1. **Jika menambah section UI baru**:
   - Buat file komponen di `frontend-diagram/src/components/ui-design/sections/Ui[Nama]Section.vue`.
   - Daftarkan komponen ke `SECTION_COMPONENTS` di `UiFrameNode.vue`.
   - Update prompt LLM di `backend-diagram/src/services/ui_design_service.go` agar AI mengenali tipe section baru tersebut.
2. **Jika menambah template UI baru**:
   - Tambahkan entri baru ke database PostgreSQL tabel `ui_templates`.
   - Pastikan field `theme`, `sections`, dan `code_export` valid JSON.
3. **Jika menambah fitur diagram baru**:
   - Modifikasi hanya modul `project_service.go` dan komponen di `src/components/nodes/`. Jangan menyentuh modul UI design.
