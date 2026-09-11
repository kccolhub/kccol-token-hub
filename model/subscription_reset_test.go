package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedSubscriptionResetPlan(t *testing.T, plan *SubscriptionPlan) {
	t.Helper()
	require.NoError(t, DB.Create(plan).Error)
}

func seedSubscriptionResetSub(t *testing.T, sub *UserSubscription) {
	t.Helper()
	require.NoError(t, DB.Create(sub).Error)
}

func getSubscriptionResetSub(t *testing.T, id int) UserSubscription {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", id).First(&sub).Error)
	return sub
}

func TestAdminResetUserSubscriptionsByPlanResetsAllActiveMatchesAndAdvancesTime(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9101,
		Title:            "Pro",
		PriceAmount:      10,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	otherPlan := &SubscriptionPlan{
		Id:               9102,
		Title:            "Basic",
		PriceAmount:      1,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	seedSubscriptionResetPlan(t, plan)
	seedSubscriptionResetPlan(t, otherPlan)

	activeEnd := now + 30*24*3600
	expiredEnd := now - 1
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9201, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 300, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9202, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 500, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9203, UserId: 101, PlanId: otherPlan.Id, AmountTotal: 100, AmountUsed: 60, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9204, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 700, StartTime: now - 7200, EndTime: expiredEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now - 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9205, UserId: 102, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 800, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9206, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 900, StartTime: now - 3600, EndTime: activeEnd, Status: "cancelled", LastResetTime: now - 3600, NextResetTime: now + 120})

	beforeReset := GetDBTimestamp()
	result, err := AdminResetUserSubscriptionsByPlan(101, plan.Id, true)
	afterReset := GetDBTimestamp()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, plan.Id, result.PlanId)
	assert.Equal(t, 2, result.MatchedCount)
	assert.Equal(t, 2, result.ResetCount)
	assert.Equal(t, 1, result.UserCount)
	assert.Equal(t, []int{101}, result.AffectedUserIds)
	assert.True(t, result.AdvanceResetTime)

	for _, id := range []int{9201, 9202} {
		sub := getSubscriptionResetSub(t, id)
		assert.Zero(t, sub.AmountUsed)
		assert.GreaterOrEqual(t, sub.LastResetTime, beforeReset)
		assert.LessOrEqual(t, sub.LastResetTime, afterReset)
		assert.Equal(t, calcNextResetTime(time.Unix(sub.LastResetTime, 0), plan, sub.EndTime), sub.NextResetTime)
	}
	assert.EqualValues(t, 60, getSubscriptionResetSub(t, 9203).AmountUsed)
	assert.EqualValues(t, 700, getSubscriptionResetSub(t, 9204).AmountUsed)
	assert.EqualValues(t, 800, getSubscriptionResetSub(t, 9205).AmountUsed)
	assert.EqualValues(t, 900, getSubscriptionResetSub(t, 9206).AmountUsed)
}

func TestAdminResetUserSubscriptionsByPlanKeepsResetTimes(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9301,
		Title:            "Team",
		PriceAmount:      20,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      2000,
		QuotaResetPeriod: SubscriptionResetMonthly,
	}
	seedSubscriptionResetPlan(t, plan)

	lastReset := now - 86400
	nextReset := now + 86400
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9302, UserId: 201, PlanId: plan.Id, AmountTotal: 2000, AmountUsed: 1200, StartTime: now - 172800, EndTime: now + 30*24*3600, Status: "active", LastResetTime: lastReset, NextResetTime: nextReset})

	result, err := AdminResetUserSubscriptionsByPlan(201, plan.Id, false)

	require.NoError(t, err)
	assert.False(t, result.AdvanceResetTime)
	sub := getSubscriptionResetSub(t, 9302)
	assert.Zero(t, sub.AmountUsed)
	assert.Equal(t, lastReset, sub.LastResetTime)
	assert.Equal(t, nextReset, sub.NextResetTime)
}

func TestAdminResetUserSubscriptionsByPlanNoActiveMatchReturnsError(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:            9401,
		Title:         "Expired",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   1000,
	}
	seedSubscriptionResetPlan(t, plan)
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9402, UserId: 301, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 500, StartTime: now - 7200, EndTime: now - 1, Status: "active"})

	result, err := AdminResetUserSubscriptionsByPlan(301, plan.Id, true)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, strings.Contains(err.Error(), "该用户没有有效的此套餐订阅"))
}

func TestAdminResetPlanSubscriptionsResetsAllActiveUsers(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9501,
		Title:            "Business",
		PriceAmount:      30,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      3000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	seedSubscriptionResetPlan(t, plan)

	activeEnd := now + 30*24*3600
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9502, UserId: 401, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1000, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9503, UserId: 401, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1100, StartTime: now - 3500, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9504, UserId: 402, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1200, StartTime: now - 3400, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9505, UserId: 403, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1300, StartTime: now - 7200, EndTime: now - 1, Status: "active", LastResetTime: now - 3600, NextResetTime: now - 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9506, UserId: 404, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1400, StartTime: now - 3600, EndTime: activeEnd, Status: "cancelled", LastResetTime: now - 3600, NextResetTime: now + 10})

	result, err := AdminResetPlanSubscriptions(plan.Id, true)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.MatchedCount)
	assert.Equal(t, 3, result.ResetCount)
	assert.Equal(t, 2, result.UserCount)
	assert.Equal(t, []int{401, 402}, result.AffectedUserIds)
	for _, id := range []int{9502, 9503, 9504} {
		sub := getSubscriptionResetSub(t, id)
		assert.Zero(t, sub.AmountUsed)
		assert.Zero(t, sub.LastResetTime)
		assert.Zero(t, sub.NextResetTime)
	}
	assert.EqualValues(t, 1300, getSubscriptionResetSub(t, 9505).AmountUsed)
	assert.EqualValues(t, 1400, getSubscriptionResetSub(t, 9506).AmountUsed)
}

func TestAdminResetPlanSubscriptionsNoMatchSucceeds(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:            9601,
		Title:         "Empty",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   1000,
	}
	seedSubscriptionResetPlan(t, plan)

	result, err := AdminResetPlanSubscriptions(plan.Id, true)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Zero(t, result.MatchedCount)
	assert.Zero(t, result.ResetCount)
	assert.Zero(t, result.UserCount)
	assert.Empty(t, result.AffectedUserIds)
}

func TestCustomSubscriptionResetAnchorBoundaries(t *testing.T) {
	anchor := time.Date(2026, 9, 15, 10, 2, 0, 0, time.FixedZone("CST", 8*3600)).Unix()
	const week = int64(7 * 24 * 3600)
	for _, tc := range []struct {
		name                            string
		base, anchor, period, end, want int64
	}{
		{"before anchor", anchor - 3600, anchor, week, 0, anchor},
		{"at anchor starts next period", anchor, anchor, week, 0, anchor + week},
		{"late purchase follows shared schedule", anchor + 3600, anchor, week, 0, anchor + week},
		{"many missed periods", anchor + 100*week + 3600, anchor, week, 0, anchor + 101*week},
		{"five hour interval", anchor + 1, anchor, 18000, 0, anchor + 18000},
		{"legacy activation based interval", anchor + 3600, 0, week, 0, anchor + 3600 + week},
		{"subscription ends before reset", anchor - 3600, anchor, week, anchor - 1, 0},
		{"reset at end is allowed", anchor - 3600, anchor, week, anchor, anchor},
		{"zero interval", anchor - 3600, anchor, 0, 0, 0},
		{"oversized interval cannot wrap", anchor, anchor, 9223372036854775807, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := &SubscriptionPlan{QuotaResetPeriod: SubscriptionResetCustom, QuotaResetCustomSeconds: tc.period, QuotaResetAnchorTime: tc.anchor}
			assert.Equal(t, tc.want, calcNextResetTime(time.Unix(tc.base, 0), plan, tc.end))
		})
	}
}

func TestSubscriptionResetAnchorPersistsAndAlignsNewPurchases(t *testing.T) {
	truncateTables(t)
	anchor := GetDBTimestamp() + 3600
	plan := &SubscriptionPlan{Title: "Anchored weekly", DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		TotalAmount: 1000, QuotaResetPeriod: SubscriptionResetCustom, QuotaResetCustomSeconds: 604800, QuotaResetAnchorTime: anchor}
	seedSubscriptionResetPlan(t, plan)
	var loaded SubscriptionPlan
	require.NoError(t, DB.First(&loaded, plan.Id).Error)
	require.Equal(t, anchor, loaded.QuotaResetAnchorTime)
	for _, userID := range []int{99001, 99002} {
		sub, err := CreateUserSubscriptionFromPlanTx(DB, userID, &loaded, "admin")
		require.NoError(t, err)
		assert.Equal(t, anchor, sub.NextResetTime)
		assert.Equal(t, int64(0), sub.AmountUsed)
	}
}

func TestAnchoredSubscriptionResetSkipsMissedPeriodsWithoutEarlyOrDuplicateReset(t *testing.T) {
	truncateTables(t)
	anchor := int64(1789437720)
	const interval = int64(18000)
	plan := &SubscriptionPlan{Title: "Anchored", QuotaResetPeriod: SubscriptionResetCustom, QuotaResetCustomSeconds: interval, QuotaResetAnchorTime: anchor}
	seedSubscriptionResetPlan(t, plan)
	sub := &UserSubscription{UserId: 99001, PlanId: plan.Id, Status: "active", StartTime: anchor - 100, EndTime: anchor + 100000*interval,
		LastResetTime: anchor - 100, NextResetTime: anchor, AmountTotal: 1000, AmountUsed: 400}
	require.NoError(t, DB.Create(sub).Error)
	require.NoError(t, maybeResetUserSubscriptionWithPlanTx(DB, sub, plan, anchor-1))
	assert.Equal(t, int64(400), sub.AmountUsed)
	now := anchor + 99999*interval + 10
	require.NoError(t, maybeResetUserSubscriptionWithPlanTx(DB, sub, plan, now))
	assert.Zero(t, sub.AmountUsed)
	assert.Equal(t, anchor+99999*interval, sub.LastResetTime)
	assert.Equal(t, anchor+100000*interval, sub.NextResetTime)
	sub.AmountUsed = 50
	require.NoError(t, DB.Save(sub).Error)
	require.NoError(t, maybeResetUserSubscriptionWithPlanTx(DB, sub, plan, now+1))
	assert.Equal(t, int64(50), sub.AmountUsed)
	require.NoError(t, resetUserSubscriptionTx(DB, sub, plan, now+2, true))
	assert.Equal(t, anchor+100000*interval, sub.NextResetTime)

	// A delayed reset worker must not advance beyond subscription expiry.
	sub.EndTime = anchor + 100000*interval + 100
	require.NoError(t, maybeResetUserSubscriptionWithPlanTx(DB, sub, plan, anchor+100010*interval))
	assert.Equal(t, anchor+100000*interval, sub.LastResetTime)
	assert.Zero(t, sub.NextResetTime)
}

// External DSNs must point to disposable test databases. Only this test's tables are created/dropped.
func TestSubscriptionResetAnchorSchemaMigration(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var dialector gorm.Dialector
			switch engine {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("SUBSCRIPTION_TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("SUBSCRIPTION_TEST_MYSQL_DSN not configured")
				}
				dialector = mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: dsn}}}
			case "postgres":
				dsn := os.Getenv("SUBSCRIPTION_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("SUBSCRIPTION_TEST_POSTGRES_DSN not configured")
				}
				dialector = postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: dsn, PreferSimpleProtocol: true}}}
			}
			db, err := gorm.Open(dialector, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			defer sqlDB.Close()
			var version string
			query := "SELECT version()"
			if engine == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database=%s version=%s", engine, version)
			recorder := &migrationSQLRecorder{}
			db = db.Session(&gorm.Session{Logger: recorder})
			for _, upgrade := range []bool{false, true} {
				name := fmt.Sprintf("subscription_anchor_test_%d", time.Now().UnixNano())
				table := db.Table(name)
				require.NoError(t, table.AutoMigrate(&SubscriptionPlan{}))
				defer db.Table(name).Migrator().DropTable(&SubscriptionPlan{})
				var existingPlanID int
				if upgrade {
					// Reproduce the previous plan schema, which differs only by this column.
					require.NoError(t, db.Table(name).Migrator().DropColumn(&SubscriptionPlan{}, "QuotaResetAnchorTime"))
					old := SubscriptionPlan{Title: "Existing plan", QuotaResetPeriod: "daily", TotalAmount: 1000}
					require.NoError(t, db.Table(name).Omit("QuotaResetAnchorTime").Create(&old).Error)
					existingPlanID = old.Id
				}
				for i := 0; i < 2; i++ {
					recorder.reset()
					require.NoError(t, db.Table(name).AutoMigrate(&SubscriptionPlan{}))
					if i > 0 {
						assert.Empty(t, recorder.schemaMutations(), "repeated migration must not alter the schema")
					}
				}
				if upgrade {
					var old SubscriptionPlan
					require.NoError(t, db.Table(name).First(&old, existingPlanID).Error)
					assert.Zero(t, old.QuotaResetAnchorTime)
					assert.Equal(t, "daily", old.QuotaResetPeriod)
					assert.Equal(t, int64(1000), old.TotalAmount)
				}
				plan := SubscriptionPlan{Title: "New plan", QuotaResetPeriod: SubscriptionResetCustom, QuotaResetCustomSeconds: 604800, QuotaResetAnchorTime: 1789437720}
				require.NoError(t, db.Table(name).Create(&plan).Error)
				var loaded SubscriptionPlan
				require.NoError(t, db.Table(name).First(&loaded, plan.Id).Error)
				assert.Equal(t, plan.QuotaResetAnchorTime, loaded.QuotaResetAnchorTime)
				assert.Error(t, db.Table(name).Create(&loaded).Error, "primary key uniqueness must survive migration")
				t.Logf("upgrade=%t: anchor persisted, old data preserved, repeat migration stable, primary key enforced", upgrade)
			}
		})
	}
}
