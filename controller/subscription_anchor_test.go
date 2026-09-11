package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionAnchorCreateValidationAndUpdatePreservation(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}))
	previousDB, previousRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB = previousDB; common.RedisEnabled = previousRedis })
	router := gin.New()
	router.POST("/plans", AdminCreateSubscriptionPlan)
	router.PUT("/plans/:id", AdminUpdateSubscriptionPlan)
	for _, tc := range []struct {
		name, period string
		anchor       int64
		valid        bool
	}{
		{"no anchor", "daily", 0, true},
		{"anchored custom", "custom", 1789437720, true},
		{"negative timestamp", "custom", -1, false},
		{"out of range timestamp", "custom", 253402300800, false},
		{"calendar period with anchor", "weekly", 1789437720, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := model.SubscriptionPlan{Title: tc.name, QuotaResetPeriod: tc.period, QuotaResetCustomSeconds: 604800, QuotaResetAnchorTime: tc.anchor}
			body, err := common.Marshal(AdminUpsertSubscriptionPlanRequest{Plan: plan})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/plans", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			var response struct {
				Success bool                   `json:"success"`
				Data    model.SubscriptionPlan `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, tc.valid, response.Success, recorder.Body.String())
			if !tc.valid {
				return
			}
			assert.Equal(t, tc.anchor, response.Data.QuotaResetAnchorTime)
			// Older clients omit the new field when editing; it must remain unchanged.
			plan.Title = "Edited title"
			plan.QuotaResetAnchorTime = 0
			body, err = common.Marshal(AdminUpsertSubscriptionPlanRequest{Plan: plan})
			require.NoError(t, err)
			req = httptest.NewRequest(http.MethodPut, "/plans/"+strconv.Itoa(response.Data.Id), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			recorder = httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			var updateResponse struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &updateResponse))
			require.True(t, updateResponse.Success, recorder.Body.String())
			var saved model.SubscriptionPlan
			require.NoError(t, db.First(&saved, response.Data.Id).Error)
			assert.Equal(t, tc.anchor, saved.QuotaResetAnchorTime)
			assert.Equal(t, "Edited title", saved.Title)
		})
	}
}
