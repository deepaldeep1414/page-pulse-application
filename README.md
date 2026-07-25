# 🚀 PagePulse

A fast and lightweight website auditing tool built with **Go** that analyzes any public webpage and returns key SEO, accessibility, and performance insights within seconds.

<p align="center">
  <a href="https://page-pulse-vddy.onrender.com"><strong>🌐 Live Demo</strong></a> •
  <a href="https://github.com/deepaldeep1414/page-pulse-application"><strong>📂 Repository</strong></a>
</p>

---

## 📸 Preview

![PagePulse Screenshot](./screenshot.png)

> Save the screenshot you uploaded as **`screenshot.png`** in the root of your repository.

---

## ✨ Features

- 🔍 Analyze any publicly accessible website
- ⚡ Measure response time
- 🌐 Check HTTP status codes
- 📄 Extract page title
- 📝 Fetch meta descriptions
- 🏷 Count H1 headings
- 🖼 Detect missing image alt text
- 📚 Calculate page word count
- ❌ Gracefully handles invalid or unreachable URLs
- 💻 Clean and responsive interface

---

## 🛠 Built With

- **Go (Golang)**
- HTML5
- CSS3
- JavaScript
- Go Standard Library
- `golang.org/x/net/html`

---

## 🌍 Live Demo

🔗 https://page-pulse-vddy.onrender.com

---

## 📂 Repository

🔗 https://github.com/deepaldeep1414/page-pulse-application

---

## 🚀 Getting Started

### Clone the repository

```bash
git clone https://github.com/deepaldeep1414/page-pulse-application.git
```

Move into the project directory

```bash
cd page-pulse-application
```

Install dependencies

```bash
go mod tidy
```

Run the server

```bash
go run .
```

Open your browser

```
http://localhost:8080
```

---

## 📡 API Endpoint

### Request

```http
GET /api/audit?url=https://example.com
```

Example

```bash
curl "http://localhost:8080/api/audit?url=https://example.com"
```

---

## 📊 Example Output

```json
{
  "status": 200,
  "responseTime": 447,
  "title": "Render - The Easiest Cloud For All Your Apps",
  "metaDescription": "...",
  "h1Count": 0,
  "imageCount": 0,
  "missingAltCount": 0,
  "wordCount": 16
}
```

---

## 📁 Project Structure

```
page-pulse-application
│
├── main.go
├── audit.go
├── audit_test.go
├── static
│   ├── index.html
│   ├── style.css
│   └── script.js
├── go.mod
├── go.sum
└── README.md
```

---

## 🧪 Run Tests

```bash
go test ./...
```

---

## 🎯 Future Improvements

- Lighthouse-like scoring
- PDF export
- SSL certificate analysis
- Core Web Vitals
- Security header checks
- Historical audit reports
- Mobile optimization analysis

---

## 🤝 Contributing

Contributions are welcome.

1. Fork the repository
2. Create a new feature branch

```bash
git checkout -b feature-name
```

3. Commit your changes

```bash
git commit -m "Add feature"
```

4. Push to GitHub

```bash
git push origin feature-name
```

5. Open a Pull Request

---

## 👨‍💻 Author

**Deepal Deep**

GitHub: https://github.com/deepaldeep1414

---

## 📄 License

This project is licensed under the MIT License.

---

⭐ If you found this project useful, consider giving it a star on GitHub!
