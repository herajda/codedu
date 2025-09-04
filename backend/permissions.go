package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

// getDockerUIDGID parses dockerUser (e.g., "65534:65534") into uid,gid.
// Returns ok=false if parsing fails.
func getDockerUIDGID() (int, int, bool) {
    parts := strings.Split(strings.TrimSpace(dockerUser), ":")
    if len(parts) != 2 {
        return 0, 0, false
    }
    uid, err1 := strconv.Atoi(parts[0])
    gid, err2 := strconv.Atoi(parts[1])
    if err1 != nil || err2 != nil {
        return 0, 0, false
    }
    return uid, gid, true
}

// ensureSandboxPerms makes all directories under root 0755, files 0644,
// and best-effort chowns to the docker user so the container user can read.
func ensureSandboxPerms(root string) error {
    uid, gid, ok := getDockerUIDGID()
    // Walk and adjust permissions; chown only if uid/gid parsed.
    return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil
        }
        if info.IsDir() {
            _ = os.Chmod(path, 0755)
        } else {
            _ = os.Chmod(path, 0644)
        }
        if ok {
            if chErr := os.Chown(path, uid, gid); chErr != nil {
                // Not fatal; log to stderr for diagnostics but continue.
                fmt.Fprintf(os.Stderr, "[perm] chown %s -> %d:%d failed: %v\n", path, uid, gid, chErr)
            }
        }
        return nil
    })
}

// ensureExecRoot prepares the shared execution root directory with permissive
// traversal so nested containers can bind-mount it regardless of UID mapping.
func ensureExecRoot(path string) {
    if strings.TrimSpace(path) == "" {
        return
    }
    _ = os.MkdirAll(path, 0777)
    _ = os.Chmod(path, 0777)
    if uid, gid, ok := getDockerUIDGID(); ok {
        _ = os.Chown(path, uid, gid)
    }
}
