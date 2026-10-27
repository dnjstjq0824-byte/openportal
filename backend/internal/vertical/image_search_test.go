package vertical

import (
	"testing"
)

func TestComputeDHashAndHammingDistance(t *testing.T) {
	var g1 [8][9]uint8
	// Fill with ascending values in row
	for r := 0; r < 8; r++ {
		for c := 0; c < 9; c++ {
			g1[r][c] = uint8(c * 10)
		}
	}

	h1 := ComputeDHashFromGradients(g1)
	if h1 == 0 {
		t.Errorf("expected non-zero hash for gradient matrix")
	}

	// Identical grid must have Hamming distance 0
	if dist := HammingDistance(h1, h1); dist != 0 {
		t.Errorf("expected distance 0, got %d", dist)
	}

	// Modify one gradient bit
	h2 := h1 ^ 0x01
	if dist := HammingDistance(h1, h2); dist != 1 {
		t.Errorf("expected distance 1, got %d", dist)
	}
}

func TestClusterNearDuplicates(t *testing.T) {
	records := []ImageFeatureRecord{
		{ImageID: "img1", DHash: 0x1111222233334444, Title: "Sunset at sea", Resolution: 1920 * 1080, QualityScore: 0.9},
		{ImageID: "img2", DHash: 0x1111222233334445, Title: "Sunset at sea (re-encoded)", Resolution: 1280 * 720, QualityScore: 0.7}, // 1 bit diff
		{ImageID: "img3", DHash: 0xF0F0F0F0F0F0F0F0, Title: "Snow Mountain", Resolution: 2048 * 1536, QualityScore: 0.95},          // completely different
	}

	clusters := ClusterNearDuplicates(records, 5)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	// First cluster should have 2 members (img1 and img2)
	var twoMemberCluster *ImageCluster
	for i := range clusters {
		if len(clusters[i].Members) == 2 {
			twoMemberCluster = &clusters[i]
			break
		}
	}
	if twoMemberCluster == nil {
		t.Fatalf("expected a 2-member cluster")
	}
	// Representative should be img1 (higher res * quality)
	if twoMemberCluster.RepresentativeImageID != "img1" {
		t.Errorf("expected img1 as representative, got %s", twoMemberCluster.RepresentativeImageID)
	}
}

func TestSearchImages(t *testing.T) {
	records := []ImageFeatureRecord{
		{ImageID: "img1", DHash: 0x1000, Title: "Jeju Island Beach", AltText: "Emerald waters of Hyeopjae", Tags: []string{"jeju", "beach"}, Resolution: 1920 * 1080, QualityScore: 0.85},
		{ImageID: "img2", DHash: 0x1001, Title: "Jeju Beach thumbnail", AltText: "Hyeopjae beach view", Tags: []string{"jeju"}, Resolution: 800 * 600, QualityScore: 0.6},
		{ImageID: "img3", DHash: 0xF0F0F0F0F0F0F0F0, Title: "Seoul Skyline at Night", AltText: "Namsan Seoul Tower view", Tags: []string{"seoul", "tower"}, Resolution: 3840 * 2160, QualityScore: 0.95},
	}

	results := SearchImages("jeju beach", records, 5)
	if len(results) == 0 {
		t.Fatalf("expected at least 1 result for 'jeju beach'")
	}
	if results[0].Record.ImageID != "img1" {
		t.Errorf("expected img1 as top result, got %s", results[0].Record.ImageID)
	}
	if results[0].ClusterSize != 2 {
		t.Errorf("expected cluster size 2, got %d", results[0].ClusterSize)
	}
}
