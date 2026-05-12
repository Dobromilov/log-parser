package parser

import (
	"archive/zip"
	"errors"
	"fmt"
	"log-parser/internal/models"
	"path"
	"path/filepath"
	"strings"
)

const dbCSVFileName = "ibdiagnet2.db_csv"

func ParseArchive(dataDir, archivePath string) (*models.ParseResult, error) {
	fullPath, err := resolveDataPath(dataDir, archivePath)
	if err != nil {
		return nil, err
	}

	reader, err := zip.OpenReader(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer reader.Close()

	dbCSVFile, err := findDBCSVFile(reader.File)
	if err != nil {
		return nil, err
	}

	file, err := dbCSVFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s from archive: %w", dbCSVFileName, err)
	}
	defer file.Close()

	return ParseDBCSV(file)
}

func resolveDataPath(dataDir, requestedPath string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("data dir is required")
	}

	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" {
		return "", errors.New("archive path is required")
	}
	if filepath.IsAbs(requestedPath) {
		return "", errors.New("archive path must be relative")
	}

	cleanRequestedPath := filepath.Clean(requestedPath)
	if cleanRequestedPath == "." || cleanRequestedPath == ".." || strings.HasPrefix(cleanRequestedPath, ".."+string(filepath.Separator)) {
		return "", errors.New("archive path escapes data dir")
	}

	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}

	realDataDir, err := filepath.EvalSymlinks(absDataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data dir symlinks: %w", err)
	}

	fullPath := filepath.Join(absDataDir, cleanRequestedPath)
	realFullPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve archive symlinks: %w", err)
	}

	relPath, err := filepath.Rel(realDataDir, realFullPath)
	if err != nil {
		return "", fmt.Errorf("resolve archive path: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", errors.New("archive path escapes data dir")
	}

	return fullPath, nil
}

func findDBCSVFile(files []*zip.File) (*zip.File, error) {
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		if path.Base(file.Name) == dbCSVFileName {
			return file, nil
		}
	}

	return nil, fmt.Errorf("%s not found in archive", dbCSVFileName)
}
