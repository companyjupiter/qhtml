package qhtml

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ChunkMediaSchemaVersion = "qhtml.chunk_media.v1"

const defaultChunkBytes int64 = 1024 * 1024

type ChunkMediaRequest struct {
	ProjectRoot   string
	LaneRoot      string
	SlotRoot      string
	StateRoot     string
	ChunkBytes    int64
	WriteEvidence bool
	ObservedAt    time.Time
}

type MediaChunk struct {
	Index  int    `json:"index"`
	Offset int64  `json:"offset"`
	Bytes  int    `json:"bytes"`
	Digest string `json:"digest"`
}

type ChunkedMediaAsset struct {
	SlotPath    string       `json:"slot_path"`
	AssetPath   string       `json:"asset_path"`
	Bytes       int64        `json:"bytes"`
	ChunkBytes  int64        `json:"chunk_bytes"`
	ChunkCount  int          `json:"chunk_count"`
	ChunkDigest string       `json:"chunk_digest"`
	MimeHint    string       `json:"mime_hint"`
	Chunks      []MediaChunk `json:"chunks"`
}

type ChunkMediaReceipt struct {
	SchemaVersion string              `json:"schema_version"`
	Status        string              `json:"status"`
	ProductID     string              `json:"product_id"`
	LaneRoot      string              `json:"lane_root"`
	SlotRoot      string              `json:"slot_root"`
	StateRoot     string              `json:"state_root"`
	ReceiptPath   string              `json:"receipt_path,omitempty"`
	LaneDigest    string              `json:"lane_digest"`
	ChunkDigest   string              `json:"chunk_digest"`
	ChunkBytes    int64               `json:"chunk_bytes"`
	AssetCount    int                 `json:"asset_count"`
	TotalBytes    int64               `json:"total_bytes"`
	Assets        []ChunkedMediaAsset `json:"assets"`
	NegativeCases []string            `json:"negative_cases"`
	Policy        string              `json:"policy"`
	ObservedAt    string              `json:"observed_at"`
	Details       map[string]string   `json:"details,omitempty"`
}

func ChunkMedia(req ChunkMediaRequest) (ChunkMediaReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return ChunkMediaReceipt{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return ChunkMediaReceipt{}, errors.New("qhtml lane root required")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	slotRootRel := strings.TrimSpace(req.SlotRoot)
	if slotRootRel == "" {
		slotRootRel = "04"
	}
	slotRootAbs := absPath(laneRoot, slotRootRel)
	slotRootRel, err = laneRelativePath(laneRoot, slotRootAbs, "qhtml media slot root must be inside lane root")
	if err != nil {
		return ChunkMediaReceipt{}, err
	}
	chunkBytes := req.ChunkBytes
	if chunkBytes <= 0 {
		chunkBytes = defaultChunkBytes
	}
	if chunkBytes < 4096 {
		return ChunkMediaReceipt{}, errors.New("qhtml chunk-media --chunk-bytes must be at least 4096")
	}
	stateRoot := chunkMediaRoot(projectRoot, req.StateRoot)
	laneDigest, _, _, _, ignored, err := digestTree(laneRoot, []string{stateRoot, filepath.Join(laneRoot, "dist")})
	if err != nil {
		return ChunkMediaReceipt{}, err
	}
	assets, totalBytes, err := collectChunkedMediaAssets(laneRoot, slotRootAbs, slotRootRel, chunkBytes)
	if err != nil {
		return ChunkMediaReceipt{}, err
	}
	chunkDigest := chunkMediaAggregateDigest(laneDigest, chunkBytes, assets)
	receiptPath := filepath.Join(stateRoot, chunkDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_chunk_media.json")
	receipt := ChunkMediaReceipt{
		SchemaVersion: ChunkMediaSchemaVersion,
		Status:        "chunk_media",
		ProductID:     "qhtml",
		LaneRoot:      slashClean(laneRoot),
		SlotRoot:      slotRootRel,
		StateRoot:     slashClean(stateRoot),
		LaneDigest:    laneDigest,
		ChunkDigest:   chunkDigest,
		ChunkBytes:    chunkBytes,
		AssetCount:    len(assets),
		TotalBytes:    totalBytes,
		Assets:        assets,
		NegativeCases: []string{
			"slot_root_escape_rejected",
			"too_small_chunk_rejected",
			"media_symlink_rejected",
			"non_media_files_ignored",
			"streaming_reader_used_instead_of_whole_file_read",
		},
		Policy:     "large_media_uses_streaming_chunk_digests; chunk_receipt_is_verifiable_without_loading_full_asset_into_memory",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"ignored":      strings.Join(ignored, ","),
			"resolver":     "qhtml_go_chunk_media_v1",
		},
	}
	if req.WriteEvidence {
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func collectChunkedMediaAssets(laneRoot, slotRootAbs, slotRootRel string, chunkBytes int64) ([]ChunkedMediaAsset, int64, error) {
	info, err := os.Stat(slotRootAbs)
	if err != nil {
		return nil, 0, err
	}
	if !info.IsDir() {
		return nil, 0, errors.New("qhtml media slot root is not a directory")
	}
	var assets []ChunkedMediaAsset
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
		chunks, digest, err := chunkDigestFile(path, chunkBytes)
		if err != nil {
			return err
		}
		assets = append(assets, ChunkedMediaAsset{
			SlotPath:    slotPath,
			AssetPath:   assetRel,
			Bytes:       info.Size(),
			ChunkBytes:  chunkBytes,
			ChunkCount:  len(chunks),
			ChunkDigest: digest,
			MimeHint:    mediaMimeHint(path),
			Chunks:      chunks,
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

func chunkDigestFile(path string, chunkBytes int64) ([]MediaChunk, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	buf := make([]byte, chunkBytes)
	chunks := []MediaChunk{}
	index := 0
	var offset int64
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			sum := sha256.Sum256(buf[:n])
			chunks = append(chunks, MediaChunk{
				Index:  index,
				Offset: offset,
				Bytes:  n,
				Digest: hex.EncodeToString(sum[:]),
			})
			index++
			offset += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, "", readErr
		}
	}
	lines := []string{ChunkMediaSchemaVersion, filepath.ToSlash(filepath.Clean(path)), strconv.FormatInt(chunkBytes, 10)}
	for _, chunk := range chunks {
		lines = append(lines, strings.Join([]string{
			strconv.Itoa(chunk.Index),
			strconv.FormatInt(chunk.Offset, 10),
			strconv.Itoa(chunk.Bytes),
			chunk.Digest,
		}, "\t"))
	}
	return chunks, sha256Hex([]byte(strings.Join(lines, "\n"))), nil
}

func chunkMediaAggregateDigest(laneDigest string, chunkBytes int64, assets []ChunkedMediaAsset) string {
	lines := []string{ChunkMediaSchemaVersion, laneDigest, strconv.FormatInt(chunkBytes, 10)}
	for _, asset := range assets {
		lines = append(lines, strings.Join([]string{
			asset.SlotPath,
			asset.AssetPath,
			asset.ChunkDigest,
			strconv.FormatInt(asset.Bytes, 10),
			asset.MimeHint,
		}, "\t"))
	}
	sort.Strings(lines[3:])
	return sha256Hex([]byte(strings.Join(lines, "\n")))
}

func chunkMediaRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "chunk_media")
}
