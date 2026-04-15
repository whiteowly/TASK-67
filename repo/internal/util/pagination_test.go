package util

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		query       string
		wantPage    int
		wantPerPage int
		wantOffset  int
	}{
		{"defaults", "", 1, 20, 0},
		{"page 2", "page=2", 2, 20, 20},
		{"custom per_page", "per_page=50", 1, 50, 0},
		{"page 3 per 10", "page=3&per_page=10", 3, 10, 20},
		{"negative page defaults to 1", "page=-1", 1, 20, 0},
		{"zero per_page defaults", "per_page=0", 1, 20, 0},
		{"over max per_page capped", "per_page=200", 1, 100, 0},
		{"invalid page NaN", "page=abc", 1, 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/?"+tt.query, nil)

			pg := ParsePagination(c)

			if pg.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", pg.Page, tt.wantPage)
			}
			if pg.PerPage != tt.wantPerPage {
				t.Errorf("PerPage = %d, want %d", pg.PerPage, tt.wantPerPage)
			}
			if pg.Offset != tt.wantOffset {
				t.Errorf("Offset = %d, want %d", pg.Offset, tt.wantOffset)
			}
		})
	}
}
