package vertical

import (
	"math/bits"
	"sort"
	"strings"
)

// ImageFeatureRecord holds visual and textual features for an image.
type ImageFeatureRecord struct {
	ImageID      string   `json:"image_id"`
	DHash        uint64   `json:"dhash"`
	Title        string   `json:"title"`
	AltText      string   `json:"alt_text"`
	Tags         []string `json:"tags"`
	Resolution   int      `json:"resolution"` // width * height
	QualityScore float64  `json:"quality_score"` // 0.0 to 1.0
}

// ImageCluster groups visually near-duplicate images.
type ImageCluster struct {
	ClusterID             string               `json:"cluster_id"`
	RepresentativeImageID string               `json:"representative_image_id"`
	DHash                 uint64               `json:"dhash"`
	Members               []ImageFeatureRecord `json:"members"`
}

// ImageSearchResult represents a scored image result.
type ImageSearchResult struct {
	Record           ImageFeatureRecord `json:"record"`
	RelevanceScore   float64            `json:"relevance_score"`
	ClusterSize      int                `json:"cluster_size"`
	IsRepresentative bool               `json:"is_representative"`
}

// ComputeDHashFromGradients calculates a 64-bit difference hash (dHash) from an 8x9 grayscale matrix.
// For each row (8 rows) and each column (8 comparisons across 9 pixels), bit is 1 if pixel[c] < pixel[c+1].
func ComputeDHashFromGradients(grid [8][9]uint8) uint64 {
	var hash uint64
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			if grid[r][c] < grid[r][c+1] {
				hash |= 1 << (r*8 + c)
			}
		}
	}
	return hash
}

// HammingDistance calculates the bitwise Hamming distance between two 64-bit dHashes.
func HammingDistance(h1, h2 uint64) int {
	return bits.OnesCount64(h1 ^ h2)
}

// ClusterNearDuplicates clusters images where Hamming distance <= maxHammingDist (typically <= 10 bits).
// Within each cluster, the representative image is chosen by highest quality_score * resolution.
func ClusterNearDuplicates(records []ImageFeatureRecord, maxHammingDist int) []ImageCluster {
	visited := make(map[string]bool)
	var clusters []ImageCluster

	for i := 0; i < len(records); i++ {
		rec := records[i]
		if visited[rec.ImageID] {
			continue
		}

		clusterMembers := []ImageFeatureRecord{rec}
		visited[rec.ImageID] = true

		for j := i + 1; j < len(records); j++ {
			other := records[j]
			if visited[other.ImageID] {
				continue
			}

			if HammingDistance(rec.DHash, other.DHash) <= maxHammingDist {
				clusterMembers = append(clusterMembers, other)
				visited[other.ImageID] = true
			}
		}

		// Pick representative: highest score = QualityScore * log(Resolution)
		best := clusterMembers[0]
		bestScore := float64(best.Resolution) * best.QualityScore
		for _, m := range clusterMembers[1:] {
			score := float64(m.Resolution) * m.QualityScore
			if score > bestScore {
				best = m
				bestScore = score
			}
		}

		clusters = append(clusters, ImageCluster{
			ClusterID:             "cluster-" + best.ImageID,
			RepresentativeImageID: best.ImageID,
			DHash:                 best.DHash,
			Members:               clusterMembers,
		})
	}

	return clusters
}

// SearchImages scores images against a query text and deduplicates via visual clusters.
func SearchImages(query string, records []ImageFeatureRecord, maxHammingDist int) []ImageSearchResult {
	clusters := ClusterNearDuplicates(records, maxHammingDist)
	qTokens := strings.Fields(strings.ToLower(query))

	var results []ImageSearchResult

	for _, cl := range clusters {
		// Evaluate the representative image
		var rep ImageFeatureRecord
		for _, m := range cl.Members {
			if m.ImageID == cl.RepresentativeImageID {
				rep = m
				break
			}
		}

		textScore := computeImageTextScore(qTokens, rep)
		if textScore > 0 {
			totalScore := textScore + rep.QualityScore*0.2
			results = append(results, ImageSearchResult{
				Record:           rep,
				RelevanceScore:   totalScore,
				ClusterSize:      len(cl.Members),
				IsRepresentative: true,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results
}

func computeImageTextScore(qTokens []string, img ImageFeatureRecord) float64 {
	titleLower := strings.ToLower(img.Title)
	altLower := strings.ToLower(img.AltText)

	score := 0.0
	for _, tok := range qTokens {
		matched := false
		if strings.Contains(titleLower, tok) {
			score += 3.0
			matched = true
		}
		if strings.Contains(altLower, tok) {
			score += 2.0
			matched = true
		}
		for _, tag := range img.Tags {
			if strings.EqualFold(tag, tok) {
				score += 2.5
				matched = true
			}
		}
		if !matched {
			// Require at least all tokens or partial match
			score -= 0.5
		}
	}

	if score < 0 {
		return 0
	}
	return score
}
