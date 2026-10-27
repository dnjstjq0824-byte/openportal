package vertical

import (
	"testing"
)

func TestPOISearchAndProximity(t *testing.T) {
	mgr := NewPOIManager()

	// Register Seoul City Hall (Jung-gu)
	_ = mgr.RegisterPlace(POIPlace{
		ID:            "poi-cityhall",
		Name:          "서울특별시청 (Seoul City Hall)",
		Category:      "관공서",
		Address:       "서울특별시 중구 세종대로 110",
		Location:      GeoPoint{Latitude: 37.566535, Longitude: 126.977969},
		IsVerified:    true,
		SourceLicense: "ODbL-1.0",
	})

	// Register Gangnam OpenPortal Hub
	_ = mgr.RegisterPlace(POIPlace{
		ID:            "poi-gangnam",
		Name:          "오픈포털 테헤란 센터 (OpenPortal Teheran Hub)",
		Category:      "IT기업",
		Address:       "서울특별시 강남구 테헤란로 152",
		Location:      GeoPoint{Latitude: 37.500000, Longitude: 127.036000},
		IsVerified:    true,
		SourceLicense: "ODbL-1.0",
	})

	// Search from City Hall (center) within 5km radius -> should only find City Hall
	cityHallCenter := GeoPoint{Latitude: 37.5665, Longitude: 126.9780}
	resultsNear := mgr.SearchPOI("", cityHallCenter, 5.0)
	if len(resultsNear) != 1 || resultsNear[0].Place.ID != "poi-cityhall" {
		t.Fatalf("expected only City Hall within 5km, got %d results", len(resultsNear))
	}

	// Search within 20km radius -> should find both, with City Hall ranked first due to proximity
	results20k := mgr.SearchPOI("", cityHallCenter, 20.0)
	if len(results20k) != 2 {
		t.Fatalf("expected 2 places within 20km, got %d", len(results20k))
	}
	if results20k[0].Place.ID != "poi-cityhall" {
		t.Errorf("expected City Hall ranked #1 due to proximity, got %s", results20k[0].Place.ID)
	}
}

func TestPOIAutocomplete(t *testing.T) {
	mgr := NewPOIManager()
	_ = mgr.RegisterPlace(POIPlace{ID: "p1", Name: "스타벅스 강남점", Location: GeoPoint{Latitude: 37.5, Longitude: 127.0}})
	_ = mgr.RegisterPlace(POIPlace{ID: "p2", Name: "스타벅스 역삼역점", Location: GeoPoint{Latitude: 37.5, Longitude: 127.03}})
	_ = mgr.RegisterPlace(POIPlace{ID: "p3", Name: "투썸플레이스 테헤란점", Location: GeoPoint{Latitude: 37.5, Longitude: 127.04}})

	suggestions := mgr.AutocompletePOI("스타벅스", 5)
	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions for '스타벅스', got %d", len(suggestions))
	}
}

func TestSubmitCorrection(t *testing.T) {
	mgr := NewPOIManager()
	_ = mgr.RegisterPlace(POIPlace{ID: "p1", Name: "오픈포털 카페", Address: "구 주소", Location: GeoPoint{Latitude: 37.5, Longitude: 127.0}})

	corr, err := mgr.SubmitCorrection("p1", "address", "구 주소", "신 주소", "acc-user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corr.Status != CorrectionPending {
		t.Errorf("expected pending status, got %s", corr.Status)
	}
	if corr.NewValue != "신 주소" {
		t.Errorf("unexpected new value: %s", corr.NewValue)
	}

	// Non-existent place
	_, err = mgr.SubmitCorrection("p-none", "phone", "", "02-123-4567", "acc-1")
	if err == nil {
		t.Errorf("expected error for non-existent place, got nil")
	}
}
