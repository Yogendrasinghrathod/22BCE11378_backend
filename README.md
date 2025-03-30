This project implements a secure file-sharing platform backend built in Go that allows users to upload, manage, and share files. The system features user authentication with JWT, file storage (S3/local), metadata management in PostgreSQL, and performance optimizations through concurrency and caching.

Features
1. User Authentication & Authorization
User registration and login with email/password

JWT token generation and validation

Secure password storage with bcrypt hashing

Protected endpoints with middleware

Token refresh mechanism

2. File Upload & Management
Concurrent file upload processing

Metadata storage in PostgreSQL

Local/S3 storage options

File access via generated URLs

Basic file operations (upload, download, delete, list)

Project Structure
file-sharing-system/
│
├── cmd/
│   └── server/
│       └── main.go         # Application entry point
│
├── internal/
│   ├── auth/               # Authentication handlers and middleware
│   ├── config/             # Configuration management
│   ├── file/               # File handlers and services
│   ├── models/             # Database models
│   ├── storage/            # Storage interfaces (S3/local)
│   └── utils/              # Utility functions
│
├── migrations/             # Database migration files
├── pkg/                    # Reusable packages
│   ├── database/           # Database connection
│   └── jwt/                # JWT utilities
│
├── .env.example            # Environment variables templateAuthentication
POST /register - Register a new user

POST /login - Login and get JWT token
├── go.mod                  # Go module file
├── Makefile                # Build commands
└── README.md               # This file




![image](https://github.com/user-attachments/assets/3f0899b2-44a6-4d4e-b396-8ea044b790eb)

![image](https://github.com/user-attachments/assets/57c55328-4138-4b07-82a4-41ba1855a4d6)


![image](https://github.com/user-attachments/assets/6a10d682-ffe1-430e-b1f0-881085fab0e2)


![Uploading image.png…]()

