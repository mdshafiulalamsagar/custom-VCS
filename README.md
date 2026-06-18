# SagarGit - Custom Version Control System 

## Project Definition
**SagarGit** is a lightweight, fully functional custom Version Control System (VCS) built entirely from scratch using Go. It demonstrates the core internal mechanics of Git, including Blob storage, Tree generation, Commit hashing, and a custom network layer for cloud synchronization. 

Instead of relying on standard `.git` architecture, this engine operates on its own `.sagargit` database, featuring a custom `.ignore` engine and a built-in automated credential manager for seamless GitHub integration.

---

## Features
* **Custom Object Database:** Stores compressed (Zlib) blobs, trees, and commits securely in `.sagargit/objects`.
* **Dynamic `.ignore` Engine:** Automatically skips specified files/folders during tree generation.
* **Built-in Credential Manager:** Saves GitHub Personal Access Tokens locally so you only have to authenticate once.
* **Direct GitHub Push:** Uses `go-git` to translate `.sagargit` data into standard Packfiles for GitHub push compatibility.

---

## One-Step Installation

You don't need to manually clone or build the project. If you have Go installed on your system, simply run this single command to install `sagargit` globally:

```bash
go install https://github.com/mdshafiulalamsagar/custom-VCS@latest
```

*(**Note:** Ensure your Go binary path is added to your system's PATH. If `sagargit` is not recognized after installation, add `export PATH=$PATH:$(go env GOPATH)/bin` to your `~/.bashrc` or `~/.zshrc` file).*

---

## Available Commands

Once installed, you can use the following commands in any of your projects:

### 1. Initialize a Repository
```bash
sagargit init
```
*Creates the `.sagargit` database and generates a default `.ignore` file.*

### 2. Commit Changes
```bash
sagargit commit -m "Your commit message here"
```
*Creates a snapshot of your current directory (respecting the `.ignore` list) and saves it securely in the custom VCS.*

### 3. Push to GitHub (First Time)
```bash
sagargit push <repo_url> <github_username> <personal_access_token>
```
*Uploads your local codebase to a remote GitHub repository. It automatically and securely caches your credentials.*

### 4. Push to GitHub (Subsequent Pushes)
```bash
sagargit push
```
*Automatically reads your cached credentials and pushes the latest commits. No need to type passwords again!*

### 5. Advanced / Under-the-hood Commands
* `sagargit hash-object <file_path>`: Hashes a file and saves it as a blob.
* `sagargit write-tree`: Generates a tree object from the current directory structure.
* `sagargit cat-file -p <hash>`: Decompresses and reads the content of a specific VCS object.

---
**Developed by [MD Shafiul Alam Sagar](https://github.com/mdshafiulalamsagar)**