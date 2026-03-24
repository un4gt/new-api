//go:build legacy_db
// +build legacy_db

package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type userPageResponse struct {
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Total    int          `json:"total"`
	Items    []model.User `json:"items"`
}

func setupUserControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate user/log tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedControllerUser(t *testing.T, db *gorm.DB, user *model.User) *model.User {
	t.Helper()
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if user.Group == "" {
		user.Group = "default"
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func newUserAPIContext(t *testing.T, method string, target string, body []byte, role int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("role", role)
	ctx.Set("id", 1)
	return ctx, recorder
}

func decodeUserAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := common.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func decodeUserPageData(t *testing.T, resp map[string]any) userPageResponse {
	t.Helper()
	pageRaw, ok := resp["data"]
	if !ok {
		t.Fatalf("response missing data field")
	}
	pageBytes, err := common.Marshal(pageRaw)
	if err != nil {
		t.Fatalf("failed to marshal data field: %v", err)
	}
	var page userPageResponse
	if err := common.Unmarshal(pageBytes, &page); err != nil {
		t.Fatalf("failed to unmarshal page response: %v", err)
	}
	return page
}

func mustBool(t *testing.T, value any) bool {
	t.Helper()
	b, ok := value.(bool)
	if !ok {
		t.Fatalf("value is not bool: %T", value)
	}
	return b
}

func mustFloat64(t *testing.T, value any) float64 {
	t.Helper()
	n, ok := value.(float64)
	if !ok {
		t.Fatalf("value is not number: %T", value)
	}
	return n
}

func TestManageUserSetsAndClearsDisabledAt(t *testing.T) {
	db := setupUserControllerTestDB(t)
	user := seedControllerUser(t, db, &model.User{
		Username: "manage_target",
		Password: "hashed",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	})

	disableBody, err := common.Marshal(map[string]any{
		"id":     user.Id,
		"action": "disable",
	})
	if err != nil {
		t.Fatalf("failed to marshal disable body: %v", err)
	}
	disableCtx, disableRecorder := newUserAPIContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		disableBody,
		common.RoleRootUser,
	)
	ManageUser(disableCtx)

	disableResp := decodeUserAPIResponse(t, disableRecorder)
	if !mustBool(t, disableResp["success"]) {
		t.Fatalf("expected disable success, got: %v", disableResp)
	}
	disabledDataBytes, _ := common.Marshal(disableResp["data"])
	var disabledData model.User
	if err := common.Unmarshal(disabledDataBytes, &disabledData); err != nil {
		t.Fatalf("failed to decode disable data: %v", err)
	}
	if disabledData.Status != common.UserStatusDisabled {
		t.Fatalf("expected disabled status, got %d", disabledData.Status)
	}
	if disabledData.DisabledAt <= 0 {
		t.Fatalf("expected disabled_at > 0, got %d", disabledData.DisabledAt)
	}

	var userAfterDisable model.User
	if err := db.First(&userAfterDisable, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if userAfterDisable.Status != common.UserStatusDisabled {
		t.Fatalf("db status should be disabled, got %d", userAfterDisable.Status)
	}
	if userAfterDisable.DisabledAt <= 0 {
		t.Fatalf("db disabled_at should be > 0, got %d", userAfterDisable.DisabledAt)
	}

	enableBody, err := common.Marshal(map[string]any{
		"id":     user.Id,
		"action": "enable",
	})
	if err != nil {
		t.Fatalf("failed to marshal enable body: %v", err)
	}
	enableCtx, enableRecorder := newUserAPIContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		enableBody,
		common.RoleRootUser,
	)
	ManageUser(enableCtx)

	enableResp := decodeUserAPIResponse(t, enableRecorder)
	if !mustBool(t, enableResp["success"]) {
		t.Fatalf("expected enable success, got: %v", enableResp)
	}
	enableDataBytes, _ := common.Marshal(enableResp["data"])
	var enableData model.User
	if err := common.Unmarshal(enableDataBytes, &enableData); err != nil {
		t.Fatalf("failed to decode enable data: %v", err)
	}
	if enableData.Status != common.UserStatusEnabled {
		t.Fatalf("expected enabled status, got %d", enableData.Status)
	}
	if enableData.DisabledAt != 0 {
		t.Fatalf("expected disabled_at reset to 0, got %d", enableData.DisabledAt)
	}

	var userAfterEnable model.User
	if err := db.First(&userAfterEnable, user.Id).Error; err != nil {
		t.Fatalf("failed to reload enabled user: %v", err)
	}
	if userAfterEnable.Status != common.UserStatusEnabled {
		t.Fatalf("db status should be enabled, got %d", userAfterEnable.Status)
	}
	if userAfterEnable.DisabledAt != 0 {
		t.Fatalf("db disabled_at should be 0 after enable, got %d", userAfterEnable.DisabledAt)
	}
}

func TestGetAllUsersSupportsStatusAndExcludeDeleted(t *testing.T) {
	db := setupUserControllerTestDB(t)
	seedControllerUser(t, db, &model.User{
		Username:   "active_user",
		Password:   "hashed",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusEnabled,
		DisabledAt: 0,
	})
	blocked := seedControllerUser(t, db, &model.User{
		Username:   "blocked_user",
		Password:   "hashed",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusDisabled,
		DisabledAt: common.GetTimestamp(),
	})
	blockedDeleted := seedControllerUser(t, db, &model.User{
		Username:   "blocked_deleted",
		Password:   "hashed",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusDisabled,
		DisabledAt: common.GetTimestamp(),
	})
	if err := db.Delete(&model.User{}, blockedDeleted.Id).Error; err != nil {
		t.Fatalf("failed to soft delete blocked user: %v", err)
	}

	ctx, recorder := newUserAPIContext(
		t,
		http.MethodGet,
		"/api/user/?p=1&page_size=20&status=2&exclude_deleted=true",
		nil,
		common.RoleAdminUser,
	)
	GetAllUsers(ctx)

	resp := decodeUserAPIResponse(t, recorder)
	if !mustBool(t, resp["success"]) {
		t.Fatalf("expected success response, got: %v", resp)
	}

	page := decodeUserPageData(t, resp)
	if page.Total != 1 {
		t.Fatalf("expected total=1, got %d", page.Total)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(page.Items))
	}
	if page.Items[0].Id != blocked.Id {
		t.Fatalf("expected blocked user id=%d, got %d", blocked.Id, page.Items[0].Id)
	}
	if page.Items[0].Status != common.UserStatusDisabled {
		t.Fatalf("expected blocked status, got %d", page.Items[0].Status)
	}
	if page.Items[0].DeletedAt.Valid {
		t.Fatalf("expected non-deleted blocked user in response")
	}
}

func TestSearchUsersSupportsStatusAndExcludeDeleted(t *testing.T) {
	db := setupUserControllerTestDB(t)
	seedControllerUser(t, db, &model.User{
		Username:   "alpha_enabled",
		Password:   "hashed",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusEnabled,
		DisabledAt: 0,
		Group:      "default",
	})
	blocked := seedControllerUser(t, db, &model.User{
		Username:   "alpha_blocked",
		Password:   "hashed",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusDisabled,
		DisabledAt: common.GetTimestamp(),
		Group:      "default",
	})
	blockedDeleted := seedControllerUser(t, db, &model.User{
		Username:   "alpha_blocked_deleted",
		Password:   "hashed",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusDisabled,
		DisabledAt: common.GetTimestamp(),
		Group:      "default",
	})
	if err := db.Delete(&model.User{}, blockedDeleted.Id).Error; err != nil {
		t.Fatalf("failed to soft delete blocked user: %v", err)
	}

	ctx, recorder := newUserAPIContext(
		t,
		http.MethodGet,
		"/api/user/search?keyword=alpha&group=default&p=1&page_size=20&status=2&exclude_deleted=true",
		nil,
		common.RoleAdminUser,
	)
	SearchUsers(ctx)

	resp := decodeUserAPIResponse(t, recorder)
	if !mustBool(t, resp["success"]) {
		t.Fatalf("expected success response, got: %v", resp)
	}

	page := decodeUserPageData(t, resp)
	if page.Total != 1 {
		t.Fatalf("expected total=1, got %d", page.Total)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(page.Items))
	}
	if page.Items[0].Id != blocked.Id {
		t.Fatalf("expected blocked user id=%d, got %d", blocked.Id, page.Items[0].Id)
	}
	if page.Items[0].Status != common.UserStatusDisabled {
		t.Fatalf("expected blocked status, got %d", page.Items[0].Status)
	}
}

func TestGetAllUsersRejectsInvalidStatusFilter(t *testing.T) {
	setupUserControllerTestDB(t)
	ctx, recorder := newUserAPIContext(
		t,
		http.MethodGet,
		"/api/user/?p=1&page_size=20&status=invalid",
		nil,
		common.RoleAdminUser,
	)
	GetAllUsers(ctx)

	resp := decodeUserAPIResponse(t, recorder)
	if mustBool(t, resp["success"]) {
		t.Fatalf("expected failure for invalid status filter")
	}
}

func TestSearchUsersRejectsInvalidStatusFilter(t *testing.T) {
	setupUserControllerTestDB(t)
	ctx, recorder := newUserAPIContext(
		t,
		http.MethodGet,
		"/api/user/search?keyword=a&group=default&p=1&page_size=20&status=invalid",
		nil,
		common.RoleAdminUser,
	)
	SearchUsers(ctx)

	resp := decodeUserAPIResponse(t, recorder)
	if mustBool(t, resp["success"]) {
		t.Fatalf("expected failure for invalid status filter")
	}
}

func TestGetAllUsersWithoutFiltersKeepsCompatibility(t *testing.T) {
	db := setupUserControllerTestDB(t)
	active := seedControllerUser(t, db, &model.User{
		Username: "compat_active",
		Password: "hashed",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	})
	deleted := seedControllerUser(t, db, &model.User{
		Username: "compat_deleted",
		Password: "hashed",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusDisabled,
	})
	if err := db.Delete(&model.User{}, deleted.Id).Error; err != nil {
		t.Fatalf("failed to soft delete user: %v", err)
	}

	ctx, recorder := newUserAPIContext(
		t,
		http.MethodGet,
		"/api/user/?p=1&page_size=20",
		nil,
		common.RoleAdminUser,
	)
	GetAllUsers(ctx)

	resp := decodeUserAPIResponse(t, recorder)
	if !mustBool(t, resp["success"]) {
		t.Fatalf("expected success response, got: %v", resp)
	}
	page := decodeUserPageData(t, resp)
	if page.Total != 2 {
		t.Fatalf("expected total=2 for compatibility, got %d", page.Total)
	}
	idSet := map[int]bool{}
	for _, item := range page.Items {
		idSet[item.Id] = true
	}
	if !idSet[active.Id] || !idSet[deleted.Id] {
		t.Fatalf("expected both active and deleted users in default query, got %+v", idSet)
	}
}

func TestManageUserResponseContainsDisabledAtNumber(t *testing.T) {
	db := setupUserControllerTestDB(t)
	user := seedControllerUser(t, db, &model.User{
		Username: "manage_response_target",
		Password: "hashed",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	})

	disableBody, err := common.Marshal(map[string]any{
		"id":     user.Id,
		"action": "disable",
	})
	if err != nil {
		t.Fatalf("failed to marshal disable body: %v", err)
	}
	ctx, recorder := newUserAPIContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		disableBody,
		common.RoleRootUser,
	)
	ManageUser(ctx)

	resp := decodeUserAPIResponse(t, recorder)
	if !mustBool(t, resp["success"]) {
		t.Fatalf("expected success response, got: %v", resp)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("response data type mismatch: %T", resp["data"])
	}
	disabledAt := mustFloat64(t, data["disabled_at"])
	if disabledAt <= 0 {
		t.Fatalf("expected response disabled_at > 0, got %v", disabledAt)
	}
}
