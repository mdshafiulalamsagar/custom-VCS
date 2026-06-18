package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sagargit <command>")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		initRepo()
	case "hash-object":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sagargit hash-object <file_path>")
			os.Exit(1)
		}
		hash := writeBlob(os.Args[2])
		fmt.Println(hash)
	case "cat-file":
		if len(os.Args) < 4 || os.Args[2] != "-p" {
			fmt.Println("Usage: sagargit cat-file -p <object_hash>")
			os.Exit(1)
		}
		catFile(os.Args[3])
	case "write-tree":
		// Load ignore list before capturing the tree state
		ignored := loadIgnoreList()
		hash := writeTree(".", ignored)
		fmt.Println(hash)
	case "commit":
		if len(os.Args) < 4 || os.Args[2] != "-m" {
			fmt.Println("Usage: sagargit commit -m \"<commit_message>\"")
			os.Exit(1)
		}
		message := os.Args[3]
		doCommit(message)
	case "push":
		if len(os.Args) < 5 {
			fmt.Println("Usage: sagargit push <repo_url> <github_username> <personal_access_token>")
			os.Exit(1)
		}
		repoURL := os.Args[2]
		username := os.Args[3]
		token := os.Args[4]
		pushRepo(repoURL, username, token)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

// initRepo initializes the repository structure and creates a default .ignore file
func initRepo() {
	gitDir := ".sagargit"
	dirsToCreate := []string{
		filepath.Join(gitDir, "objects"),
		filepath.Join(gitDir, "refs", "heads"),
	}

	for _, dir := range dirsToCreate {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			fmt.Printf("Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	headPath := filepath.Join(gitDir, "HEAD")
	os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644)

	// Automatically create a default .ignore file if it does not exist
	ignorePath := ".ignore"
	if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
		defaultIgnoreContent := "# Files to be ignored by sagargit\n.sagargit\n.git\nsagargit\n.ignore\n"
		os.WriteFile(ignorePath, []byte(defaultIgnoreContent), 0644)
		fmt.Println("Created default .ignore file")
	}

	fmt.Println("Initialized empty sagargit repository in .sagargit/")
}

// loadIgnoreList reads the .ignore file and maps files/folders that should be skipped
func loadIgnoreList() map[string]bool {
	ignored := map[string]bool{
		".sagargit": true,
		".git":      true,
	}

	content, err := os.ReadFile(".ignore")
	if err != nil {
		// Return basic defaults if file is missing
		return ignored
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			ignored[line] = true
		}
	}

	return ignored
}

func writeBlob(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	header := fmt.Sprintf("blob %d\x00", len(content))
	fullData := append([]byte(header), content...)

	return saveObject(fullData)
}

// writeTree recursively builds tree objects while honoring the ignore list map
func writeTree(dirPath string, ignored map[string]bool) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		os.Exit(1)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var treeData []byte

	for _, entry := range entries {
		name := entry.Name()
		
		// Dynamic check against the loaded ignore map
		if ignored[name] {
			continue
		}

		fullPath := filepath.Join(dirPath, name)
		var mode string
		var hashString string

		if entry.IsDir() {
			mode = "40000"
			hashString = writeTree(fullPath, ignored) // Pass the ignore map down recursively
		} else {
			mode = "100644"
			hashString = writeBlob(fullPath)
		}

		hashBytes, _ := hex.DecodeString(hashString)
		entryData := fmt.Sprintf("%s %s\x00", mode, name)
		treeData = append(treeData, []byte(entryData)...)
		treeData = append(treeData, hashBytes...)
	}

	header := fmt.Sprintf("tree %d\x00", len(treeData))
	fullData := append([]byte(header), treeData...)

	return saveObject(fullData)
}

func doCommit(message string) {
	headContent, err := os.ReadFile(".sagargit/HEAD")
	if err != nil {
		fmt.Println("Error reading HEAD. Is it a sagargit repo?")
		os.Exit(1)
	}
	
	refPath := strings.TrimSpace(strings.Split(string(headContent), ": ")[1])
	fullRefPath := filepath.Join(".sagargit", refPath)

	var parentHash string
	parentContent, err := os.ReadFile(fullRefPath)
	if err == nil {
		parentHash = strings.TrimSpace(string(parentContent))
	}

	// Dynamic ignore checking integrated into automated commit flow
	ignored := loadIgnoreList()
	treeHash := writeTree(".", ignored)

	commitHash := commitTree(treeHash, parentHash, message)

	err = os.MkdirAll(filepath.Dir(fullRefPath), 0755)
	if err == nil {
		os.WriteFile(fullRefPath, []byte(commitHash+"\n"), 0644)
	}

	fmt.Printf("[%s] %s\n", commitHash[:7], message)
}

func commitTree(treeHash, parentHash, message string) string {
	authorName := "Sagar Bhai"
	authorEmail := "sagar@scalerverse.com"
	
	now := time.Now()
	timestamp := now.Unix()
	_, offset := now.Zone()
	tzStr := fmt.Sprintf("%+03d%02d", offset/3600, (offset%3600)/60)

	authorString := fmt.Sprintf("%s <%s> %d %s", authorName, authorEmail, timestamp, tzStr)

	var contentBuffer bytes.Buffer
	contentBuffer.WriteString(fmt.Sprintf("tree %s\n", treeHash))
	
	if parentHash != "" {
		contentBuffer.WriteString(fmt.Sprintf("parent %s\n", parentHash))
	}
	
	contentBuffer.WriteString(fmt.Sprintf("author %s\n", authorString))
	contentBuffer.WriteString(fmt.Sprintf("committer %s\n", authorString))
	contentBuffer.WriteString("\n")
	contentBuffer.WriteString(message)
	contentBuffer.WriteString("\n")

	content := contentBuffer.Bytes()
	header := fmt.Sprintf("commit %d\x00", len(content))
	fullData := append([]byte(header), content...)

	return saveObject(fullData)
}

func saveObject(fullData []byte) string {
	hasher := sha1.New()
	hasher.Write(fullData)
	hashString := hex.EncodeToString(hasher.Sum(nil))

	dirName := hashString[:2]
	fileName := hashString[2:]
	objectDir := filepath.Join(".sagargit", "objects", dirName)
	objectPath := filepath.Join(objectDir, fileName)

	if _, err := os.Stat(objectPath); err == nil {
		return hashString
	}

	os.MkdirAll(objectDir, 0755)
	var compressedData bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressedData)
	zlibWriter.Write(fullData)
	zlibWriter.Close()

	os.WriteFile(objectPath, compressedData.Bytes(), 0644)
	return hashString
}

func catFile(hash string) {
	if len(hash) != 40 {
		fmt.Println("Error: Invalid hash length")
		os.Exit(1)
	}
	objectPath := filepath.Join(".sagargit", "objects", hash[:2], hash[2:])
	file, err := os.Open(objectPath)
	if err != nil {
		fmt.Printf("Error opening object: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()
	zlibReader, _ := zlib.NewReader(file)
	defer zlibReader.Close()
	decompressedData, _ := io.ReadAll(zlibReader)
	parts := bytes.SplitN(decompressedData, []byte{0}, 2)
	if len(parts) >= 2 {
		fmt.Print(string(parts[1]))
	}
}

func pushRepo(repoURL, username, token string) {
	fmt.Println("Preparing to push to remote...")

	dotFS := osfs.New(".sagargit")
	worktreeFS := osfs.New(".")
	storage := filesystem.NewStorage(dotFS, cache.NewObjectLRUDefault())

	repo, err := git.Open(storage, worktreeFS)
	if err != nil {
		fmt.Printf("Error opening local repository: %v\n", err)
		os.Exit(1)
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})
	if err != nil && err != git.ErrRemoteExists {
		fmt.Printf("Error configuring remote: %v\n", err)
		os.Exit(1)
	}

	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: username,
			Password: token,
		},
		Progress: os.Stdout,
	})

	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			fmt.Println("Everything is already up-to-date.")
		} else {
			fmt.Printf("Failed to push: %v\n", err)
		}
	} else {
		fmt.Println("Push completed successfully!")
	}
}