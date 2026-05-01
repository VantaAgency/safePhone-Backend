// Package main is the entry point for the SafePhone backend server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/cache"
	"github.com/cherif-safephone/safephone-backend/internal/config"
	"github.com/cherif-safephone/safephone-backend/internal/database"
	"github.com/cherif-safephone/safephone-backend/internal/dexpay"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/handler"
	mw "github.com/cherif-safephone/safephone-backend/internal/middleware"
	"github.com/cherif-safephone/safephone-backend/internal/repository"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

func main() {
	// Load .env file in development
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup structured logging
	logLevel := slog.LevelInfo
	if cfg.IsDevelopment() {
		logLevel = slog.LevelDebug
	}
	if cfg.IsDevelopment() {
		slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			Level:      logLevel,
			TimeFormat: "15:04:05",
		})))
	} else {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to PostgreSQL")

	// Connect to Redis
	redisClient, err := cache.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	slog.Info("connected to Redis")

	// Initialize repositories
	userRepo := repository.NewUserRepository(pool)
	adminRepo := repository.NewAdminRepository(pool)
	employeeRepo := repository.NewEmployeeRepository(pool)
	dashboardRepo := repository.NewDashboardRepository(pool)
	planRepo := repository.NewPlanRepository(pool)
	deviceRepo := repository.NewDeviceRepository(pool)
	subRepo := repository.NewSubscriptionRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)
	contactRepo := repository.NewContactRepository(pool)
	partnerAppRepo := repository.NewPartnerApplicationRepository(pool)
	partnerRepo := repository.NewPartnerRepository(pool)
	commercialRepo := repository.NewCommercialRepository(pool)
	repairRepo := repository.NewRepairRepository(pool)
	webhookEventRepo := repository.NewWebhookEventRepository(pool)

	if cfg.IsDevelopment() {
		if err := planRepo.EnsureDevelopmentTestPlan(ctx); err != nil {
			slog.Error("failed to ensure development test plan", "error", err)
			os.Exit(1)
		}
		slog.Info("development test plan ensured", "slug", domain.DevelopmentTestPlanSlug)
	}

	// Initialize DEXPAY client (nil when not configured)
	var dexpayClient *dexpay.Client
	if cfg.DexpayEnabled() {
		dexpayClient = dexpay.NewClient(cfg.DexpayBaseURL, cfg.DexpayAPIKey, cfg.ExternalHTTPTimeout)
		slog.Info("DEXPAY payment gateway enabled",
			"environment", cfg.DexpayEnvironment(),
			"base_url", cfg.DexpayBaseURL,
			"api_key_present", cfg.DexpayAPIKey != "",
			"api_secret_present", cfg.DexpayAPISecret != "",
			"backend_public_url", cfg.BackendPublicURL,
			"frontend_url", cfg.FrontendURL,
			"credential_mode", cfg.DexpayCredentialMode(),
		)
	} else {
		if cfg.IsDevelopment() {
			slog.Warn("DEXPAY not configured, using mock payment mode")
		} else {
			slog.Warn("DEXPAY not configured; payment checkout requests will fail until backend credentials are set")
		}
	}

	// Initialize services
	userSvc := service.NewUserService(userRepo)
	adminSvc := service.NewAdminService(adminRepo)
	dashboardSvc := service.NewDashboardService(dashboardRepo, adminRepo, partnerRepo, cfg.FrontendURL)
	planSvc := service.NewPlanService(planRepo, cfg.IsDevelopment())
	deviceSvc := service.NewDeviceService(deviceRepo, subRepo)
	subSvc := service.NewSubscriptionService(subRepo, planRepo, cfg.IsDevelopment())
	claimSvc := service.NewClaimService(claimRepo, deviceRepo, subRepo)
	paymentSvc := service.NewPaymentService(paymentRepo, subRepo, planRepo, userRepo, deviceRepo, partnerRepo, commercialRepo, webhookEventRepo, dexpayClient, pool, cfg.FrontendURL, cfg.BackendPublicURL, cfg.IsDevelopment())
	contactSvc := service.NewContactService(contactRepo)
	partnerAppSvc := service.NewPartnerApplicationService(partnerAppRepo, userRepo, partnerRepo, commercialRepo, pool)
	partnerSvc := service.NewPartnerService(partnerRepo, userRepo, paymentRepo, cfg.FrontendURL)
	commercialSvc := service.NewCommercialService(commercialRepo, partnerRepo, cfg.FrontendURL)
	repairSvc := service.NewRepairService(repairRepo)
	employeeSvc := service.NewEmployeeService(employeeRepo, userRepo, subRepo, claimRepo, repairSvc)

	// Initialize handlers
	healthH := handler.NewHealthHandler(pool, redisClient)
	userH := handler.NewUserHandler(userSvc)
	adminH := handler.NewAdminHandler(adminSvc)
	employeeH := handler.NewEmployeeHandler(employeeSvc)
	dashboardH := handler.NewDashboardHandler(dashboardSvc)
	planH := handler.NewPlanHandler(planSvc)
	deviceH := handler.NewDeviceHandler(deviceSvc)
	subH := handler.NewSubscriptionHandler(subSvc)
	claimH := handler.NewClaimHandler(claimSvc)
	paymentH := handler.NewPaymentHandler(paymentSvc)
	contactH := handler.NewContactHandler(contactSvc)
	partnerAppH := handler.NewPartnerApplicationHandler(partnerAppSvc)
	partnerH := handler.NewPartnerHandler(partnerSvc)
	commercialH := handler.NewCommercialHandler(commercialSvc)
	repairH := handler.NewRepairHandler(repairSvc)
	webhookH := handler.NewWebhookHandler(paymentSvc, cfg.DexpayAPISecret, cfg.IsDevelopment())

	// Initialize auth
	jwtVerifier := auth.NewJWTVerifier(cfg.JWKSURL, cfg.JWTIssuer, redisClient)
	slog.Info("JWT auth configured", "jwks_url", cfg.JWKSURL, "issuer", cfg.JWTIssuer)

	// Initialize rate limiter
	rateLimiter := mw.NewRateLimiter(redisClient, cfg.RateLimitGeneral)

	// Build router
	r := chi.NewRouter()

	// Global middleware chain
	r.Use(mw.SecurityHeaders)
	r.Use(mw.RequestID)
	r.Use(mw.Logger)
	r.Use(mw.CORSHandler(cfg.CORSOrigins).Handler)
	r.Use(rateLimiter.Limit)

	// Health endpoints (no auth)
	r.Get("/health/live", healthH.Live)
	r.Get("/health/ready", healthH.Ready)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Get("/plans", planH.List)
		r.Post("/contact", contactH.Submit)
		r.Get("/partner-invitations/{token}", partnerH.GetInvitation)
		r.Get("/partner-referrals/{code}", partnerH.GetReferral)
		r.Post("/partner-referrals/{code}/visits", partnerH.TrackReferralVisit)
		r.Patch("/partner-clients/{id}/status", partnerH.UpdateClientStatus)
		r.With(jwtVerifier.AuthenticateOptional).Post("/repairs", repairH.CreateBooking)
		r.Post("/repairs/lookup", repairH.LookupBooking)

		// Webhook endpoint (public, signature-verified internally)
		r.Post("/webhooks/dexpay", webhookH.HandleDexpay)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(jwtVerifier.Authenticate)

			// Users
			r.Patch("/users/me", userH.UpdateProfile)

			// Partner applications (authenticated)
			r.Post("/partner-applications", partnerAppH.Submit)
			r.Get("/partner-applications/mine", partnerAppH.GetMyApplication)

			// Devices (creation is handled atomically via POST /payments)
			r.Get("/devices", deviceH.List)
			r.Get("/devices/{id}", deviceH.Get)
			r.Put("/devices/{id}", deviceH.Update)
			r.Delete("/devices/{id}", deviceH.Delete)

			// Subscriptions (creation is handled atomically via POST /payments)
			r.Get("/subscriptions", subH.List)
			r.Get("/subscriptions/{id}", subH.Get)
			r.Post("/subscriptions/{id}/cancel", subH.Cancel)

			// Claims
			r.Post("/claims", claimH.Create)
			r.Get("/claims", claimH.List)
			r.Get("/claims/{id}", claimH.Get)

			// Repairs
			r.Get("/repairs/mine", repairH.ListMine)

			// Dashboard summary
			r.Get("/dashboard/summary", dashboardH.MemberSummary)

			// Payments
			r.Post("/payments", paymentH.Create)
			r.Post("/payments/renew-subscription", paymentH.RenewSubscription)
			r.Get("/payments", paymentH.List)
			r.Get("/payments/{id}", paymentH.Get)
			r.Get("/payments/{id}/checkout", paymentH.GetCheckout)
			r.Post("/payments/{id}/resume", paymentH.Resume)

			// Partner routes
			r.Route("/partner", func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RolePartner, auth.RoleAdmin))

				r.Get("/profile", partnerH.GetProfile)
				r.Get("/overview", dashboardH.PartnerOverview)
				r.Get("/clients", partnerH.ListClients)
				r.Post("/clients", partnerH.CreateClient)
				r.Post("/clients/{id}/refresh-invitation", partnerH.RefreshInvitation)
				r.Get("/sales", partnerH.ListSales)
				r.Get("/payouts", partnerH.ListPayouts)
			})

			r.Post("/partner-invitations/{token}/claim", partnerH.ClaimInvitation)
			r.Post("/partner-referrals/{code}/claim", partnerH.ClaimReferral)

			// Commercial routes
			r.Route("/commercial", func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleCommercial, auth.RoleAdmin))

				r.Get("/overview", commercialH.Overview)
				r.Get("/partners", commercialH.ListPartners)
				r.Get("/commissions", commercialH.ListCommissions)
				r.Get("/activity-reports", commercialH.ListActivityReports)
				r.Post("/activity-reports", commercialH.CreateActivityReport)
				r.Get("/activity-reports/{id}/photo", commercialH.ActivityReportPhoto)
			})

			// Admin routes
			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleAdmin))

				r.Get("/stats", adminH.Stats)
				r.Get("/overview", dashboardH.AdminOverview)
				r.Get("/customers", adminH.ListCustomers)
				r.Get("/payments", adminH.ListPayments)
				r.Get("/employees", adminH.ListEmployees)
				r.Get("/employees/{id}", adminH.GetEmployee)
				r.Patch("/employees/{id}/status", adminH.UpdateEmployeeStatus)
				r.Get("/commercials", commercialH.AdminListCommercials)
				r.Get("/commercials/activity-reports", commercialH.AdminListActivityReports)
				r.Get("/commercials/{id}", commercialH.AdminGetCommercial)
				r.Patch("/commercials/{id}/status", commercialH.AdminUpdateStatus)
				r.Patch("/commercials/{id}/commission", commercialH.AdminUpdateCommission)
				r.Get("/claims", claimH.AdminList)
				r.Put("/claims/{id}/status", claimH.AdminUpdateStatus)
				r.Get("/repairs", repairH.AdminList)
				r.Get("/repairs/{id}", repairH.AdminGet)
				r.Post("/repairs/{id}/accept", repairH.AdminAccept)
				r.Post("/repairs/{id}/reject", repairH.AdminReject)
				r.Put("/repairs/{id}/status", repairH.AdminUpdateStatus)
				r.Put("/repairs/{id}/amount", repairH.AdminUpdateAmount)
				r.Get("/partners", partnerH.ListAllPartners)
				r.Get("/partners/{id}/commissions", partnerH.ListAdminPartnerCommissions)
				r.Get("/partners/{id}/referrals", partnerH.ListAdminPartnerReferrals)
				r.Get("/partner-applications", partnerAppH.AdminList)
				r.Put("/partner-applications/{id}/review", partnerAppH.AdminReview)
			})

			// Employee routes
			r.Route("/employee", func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleEmployee))

				r.Get("/overview", employeeH.Overview)
				r.Get("/clients", employeeH.ListClients)
				r.Get("/clients/{id}", employeeH.GetClient)
				r.Get("/payment-follow-ups", employeeH.ListPaymentFollowUps)
				r.Get("/claims", employeeH.ListClaims)
				r.Get("/claims/{id}", employeeH.GetClaim)
				r.Patch("/claims/{id}/status", employeeH.UpdateClaimStatus)
				r.Get("/repairs", employeeH.ListRepairs)
				r.Get("/repairs/{id}", employeeH.GetRepair)
				r.Put("/repairs/{id}/status", employeeH.UpdateRepairStatus)
				r.Put("/repairs/{id}/amount", employeeH.UpdateRepairAmount)
				r.Get("/tasks", employeeH.ListTasks)
				r.Get("/follow-ups", employeeH.GetFollowUp)
				r.Put("/follow-ups", employeeH.UpsertFollowUp)
				r.Get("/notes", employeeH.ListNotes)
				r.Post("/notes", employeeH.CreateNote)
			})
		})
	})

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("server shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
