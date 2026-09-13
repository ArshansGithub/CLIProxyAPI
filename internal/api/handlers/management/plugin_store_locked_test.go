//go:build locked

package management

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPluginStoreRefusedUnderLocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	for _, fn := range []func(*gin.Context){h.ListPluginStore, h.InstallPluginFromStore} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		fn(c)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	}
}
