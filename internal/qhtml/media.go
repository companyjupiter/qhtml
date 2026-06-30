package qhtml

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MediaSchemaVersion = "qhtml.media_resolver.v1"

const defaultMediaMaxBytes int64 = 25 * 1024 * 1024

type MediaRequest struct {
	ProjectRoot   string
	LaneRoot      string
	SlotRoot      string
	OutDir        string
	StateRoot     string
	MaxBytes      int64
	WriteEvidence bool
	ObservedAt    time.Time
}

type MediaAsset struct {
	SlotPath   string `json:"slot_path"`
	AssetPath  string `json:"asset_path"`
	ExportPath string `json:"export_path,omitempty"`
	Digest     string `json:"digest"`
	Bytes      int64  `json:"bytes"`
	MimeHint   string `json:"mime_hint"`
}

type MediaReceipt struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	LaneRoot      string            `json:"lane_root"`
	SlotRoot      string            `json:"slot_root"`
	OutDir        string            `json:"out_dir,omitempty"`
	StateRoot     string            `json:"state_root"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	LaneDigest    string            `json:"lane_digest"`
	MediaDigest   string            `json:"media_digest"`
	AssetCount    int               `json:"asset_count"`
	TotalBytes    int64             `json:"total_bytes"`
	MaxBytes      int64             `json:"max_bytes"`
	Assets        []MediaAsset      `json:"assets"`
	NegativeCases []string          `json:"negative_cases"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

func ResolveMedia(req MediaRequest) (MediaReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return MediaReceipt{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return MediaReceipt{}, errors.New("qhtml lane root required")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	slotRootRel := strings.TrimSpace(req.SlotRoot)
	if slotRootRel == "" {
		slotRootRel = "04"
	}
	slotRootAbs := absPath(laneRoot, slotRootRel)
	slotRootRel, err = laneRelativePath(laneRoot, slotRootAbs, "qhtml media slot root must be inside lane root")
	if err != nil {
		return MediaReceipt{}, err
	}
	outDir := ""
	if strings.TrimSpace(req.OutDir) != "" {
		outDir = absPath(projectRoot, req.OutDir)
		if err := validateMediaOutDir(laneRoot, outDir); err != nil {
			return MediaReceipt{}, err
		}
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMediaMaxBytes
	}
	stateRoot := mediaRoot(projectRoot, req.StateRoot)
	laneDigest, _, _, _, ignored, err := digestTree(laneRoot, []string{stateRoot, filepath.Join(laneRoot, "dist")})
	if err != nil {
		return MediaReceipt{}, err
	}
	assets, totalBytes, err := collectMediaAssets(laneRoot, slotRootAbs, slotRootRel, outDir, maxBytes)
	if err != nil {
		return MediaReceipt{}, err
	}
	mediaDigest := mediaAssetsDigest(assets, laneDigest, maxBytes)
	receiptPath := filepath.Join(stateRoot, mediaDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_media.json")
	receipt := MediaReceipt{
		SchemaVersion: MediaSchemaVersion,
		Status:        "media_resolved",
		ProductID:     "qhtml",
		LaneRoot:      slashClean(laneRoot),
		SlotRoot:      slotRootRel,
		OutDir:        slashClean(outDir),
		StateRoot:     slashClean(stateRoot),
		LaneDigest:    laneDigest,
		MediaDigest:   mediaDigest,
		AssetCount:    len(assets),
		TotalBytes:    totalBytes,
		MaxBytes:      maxBytes,
		Assets:        assets,
		NegativeCases: []string{
			"slot_root_escape_rejected",
			"out_dir_inside_lane_rejected_except_dist",
			"oversize_media_rejected",
			"media_symlink_rejected",
			"non_media_files_ignored",
		},
		Policy:     "media_slots_are_lane_relative; assets_are_digest_bound; exported_media_is_projection_cache_not_source_truth",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"ignored":      strings.Join(ignored, ","),
			"resolver":     "qhtml_go_media_slot_resolver_v1",
		},
	}
	if req.WriteEvidence {
		if outDir != "" {
			for _, asset := range assets {
				if err := copyMediaAsset(laneRoot, outDir, asset); err != nil {
					return receipt, err
				}
			}
		}
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func collectMediaAssets(laneRoot, slotRootAbs, slotRootRel, outDir string, maxBytes int64) ([]MediaAsset, int64, error) {
	info, err := os.Stat(slotRootAbs)
	if err != nil {
		return nil, 0, err
	}
	if !info.IsDir() {
		return nil, 0, errors.New("qhtml media slot root is not a directory")
	}
	var assets []MediaAsset
	var totalBytes int64
	err = filepath.WalkDir(slotRootAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == slotRootAbs {
			return nil
		}
		if shouldIgnorePath(slotRootAbs, path, d, nil) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("qhtml media symlink not allowed")
		}
		if !isMediaAssetPath(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxBytes {
			return errors.New("qhtml media asset exceeds max bytes")
		}
		assetRel, err := filepath.Rel(laneRoot, path)
		if err != nil {
			return err
		}
		assetRel = filepath.ToSlash(filepath.Clean(assetRel))
		slotRel, err := filepath.Rel(slotRootAbs, filepath.Dir(path))
		if err != nil {
			return err
		}
		slotPath := filepath.ToSlash(filepath.Clean(filepath.Join(slotRootRel, slotRel)))
		if slotRel == "." {
			slotPath = slotRootRel
		}
		digest, err := digestFile(path)
		if err != nil {
			return err
		}
		exportPath := ""
		if outDir != "" {
			exportPath = slashClean(filepath.Join(outDir, filepath.FromSlash(assetRel)))
		}
		assets = append(assets, MediaAsset{
			SlotPath:   slotPath,
			AssetPath:  assetRel,
			ExportPath: exportPath,
			Digest:     digest,
			Bytes:      info.Size(),
			MimeHint:   mediaMimeHint(path),
		})
		totalBytes += info.Size()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].SlotPath == assets[j].SlotPath {
			return assets[i].AssetPath < assets[j].AssetPath
		}
		return assets[i].SlotPath < assets[j].SlotPath
	})
	return assets, totalBytes, nil
}

func mediaAssetsDigest(assets []MediaAsset, laneDigest string, maxBytes int64) string {
	lines := []string{MediaSchemaVersion, laneDigest, strconv.FormatInt(maxBytes, 10)}
	for _, asset := range assets {
		lines = append(lines, strings.Join([]string{
			asset.SlotPath,
			asset.AssetPath,
			asset.Digest,
			strconv.FormatInt(asset.Bytes, 10),
			asset.MimeHint,
		}, "\t"))
	}
	sort.Strings(lines[3:])
	return sha256Hex([]byte(strings.Join(lines, "\n")))
}

func copyMediaAsset(laneRoot, outDir string, asset MediaAsset) error {
	sourcePath := filepath.Join(laneRoot, filepath.FromSlash(asset.AssetPath))
	targetPath := filepath.Join(outDir, filepath.FromSlash(asset.AssetPath))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
		return err
	}
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := targetPath + ".tmp"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, targetPath)
}

func validateMediaOutDir(laneRoot, outDir string) error {
	rel, err := filepath.Rel(laneRoot, outDir)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel != ".." && !strings.HasPrefix(rel, "../") {
		if rel == "dist" || strings.HasPrefix(rel, "dist/") {
			return nil
		}
		return errors.New("qhtml media out-dir inside lane root must be under dist/")
	}
	return nil
}

func laneRelativePath(laneRoot, path, message string) (string, error) {
	rel, err := filepath.Rel(laneRoot, path)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errors.New(message)
	}
	return rel, nil
}

func isMediaAssetPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".apng", ".avif", ".gif", ".jpg", ".jpeg", ".png", ".svg", ".webp", ".bmp", ".ico", ".mp4", ".webm", ".mov", ".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac":
		return true
	default:
		return false
	}
}

func mediaMimeHint(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".apng":
		return "image/apng"
	case ".avif":
		return "image/avif"
	case ".gif":
		return "image/gif"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	default:
		return "application/octet-stream"
	}
}

func mediaRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "media")
}
