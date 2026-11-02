package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spacelift-solutions/Mayday/internal/control"
	"github.com/spacelift-solutions/Mayday/internal/license"
	"github.com/spacelift-solutions/Mayday/internal/model"
	"github.com/spacelift-solutions/Mayday/internal/target"
	"github.com/spacelift-solutions/Mayday/internal/web"
)

type stopService struct {
	client             *ssm.Client
	instance, document string
}

func (s stopService) Stop(ctx context.Context) error {
	_, err := s.client.SendCommand(ctx, &ssm.SendCommandInput{InstanceIds: []string{s.instance}, DocumentName: aws.String(s.document), Comment: aws.String("Mayday controlled service outage"), TimeoutSeconds: aws.Int32(30)})
	return err
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func number(key string, fallback, min, max int) (int, error) {
	v, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || v < min || v > max {
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return v, nil
}

func run(ctx context.Context) error {
	if len(os.Args) != 2 {
		return errors.New("usage: mayday target|monitor|diagnose")
	}
	if os.Args[1] == "diagnose" {
		return diagnose(ctx)
	}
	var store license.Store
	var faults control.Faults
	bucket, key := os.Getenv("LICENSE_BUCKET"), env("LICENSE_KEY", "license/demo-license.json")
	if path := os.Getenv("LICENSE_FILE"); path != "" {
		store = license.File{Path: path}
	} else {
		if bucket == "" {
			return errors.New("LICENSE_BUCKET or local LICENSE_FILE is required")
		}
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return err
		}
		store = license.S3{Client: s3.NewFromConfig(cfg), Bucket: bucket, Key: key}
		if instance := os.Getenv("TARGET_INSTANCE_ID"); instance != "" {
			document := os.Getenv("STOP_DOCUMENT_NAME")
			if document == "" {
				return errors.New("STOP_DOCUMENT_NAME is required with TARGET_INSTANCE_ID")
			}
			faults = stopService{client: ssm.NewFromConfig(cfg), instance: instance, document: document}
		}
	}
	var handler http.Handler
	var addr string
	switch os.Args[1] {
	case "target":
		workers, err := number("WORKERS", 2, 1, 32)
		if err != nil {
			return err
		}
		queue, err := number("QUEUE_LIMIT", 8, 0, 256)
		if err != nil {
			return err
		}
		ms, err := number("WORK_MS", 200, 1, 10000)
		if err != nil {
			return err
		}
		s := target.New(store, workers, queue, time.Duration(ms)*time.Millisecond, bucket, key)
		s.Check(ctx)
		go s.Watch(ctx)
		handler = s.Handler()
		addr = env("LISTEN_ADDR", ":8081")
	case "monitor":
		s, err := control.New(control.Config{TargetURL: env("TARGET_URL", "http://127.0.0.1:8081"), Password: os.Getenv("DEMO_PASSWORD"), SecureCookies: env("SESSION_COOKIE_SECURE", "false") == "true", WebhookURL: os.Getenv("FLOWS_WEBHOOK_URL"), WebhookToken: os.Getenv("FLOWS_WEBHOOK_TOKEN"), StatePath: env("STATE_PATH", ".local/monitor.json"), StackID: env("SERVICE_STACK_ID", "mayday-service")}, store, faults)
		if err != nil {
			return err
		}
		go s.Run(ctx)
		handler = s.Handler(web.Handler())
		addr = env("LISTEN_ADDR", ":8080")
	default:
		return errors.New("usage: mayday target|monitor|diagnose")
	}
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(c); err != nil {
			slog.Error("HTTP shutdown", "error", err)
		}
	}()
	slog.Info("Mayday listening", "mode", os.Args[1], "address", addr)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func diagnose(ctx context.Context) error {
	u := strings.TrimRight(env("TARGET_URL", "http://127.0.0.1:8081"), "/")
	r, err := http.NewRequestWithContext(ctx, "GET", u+"/health", nil)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(r)
	if err != nil {
		return fmt.Errorf("service endpoint unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var h model.Health
	if err = json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(&h); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(h); err != nil {
		return err
	}
	fmt.Println("Expected entitlement: service=mayday, license_key=DEMO-ONLY, enabled=true. Resource ownership: bucket=OpenTofu; license object=Intent. Inspect the Mayday Intent project's policy and current object state before repair.")
	if h.License == "invalid" {
		fmt.Println("S3 content repair: refresh the existing Intent resource, then update that same resource with valid license content and etag set to the MD5 of those exact bytes. S3 refresh reads metadata, not the content body; the explicit desired etag ensures an upload even if remembered content already looks valid. Keep the bucket/key, empty ACL, and AES256 encryption.")
	}
	if resp.StatusCode != 200 || h.Status != "healthy" {
		return errors.New("service dependency check failed; evidence printed above")
	}
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		slog.Error("Mayday exited", "error", err)
		os.Exit(1)
	}
}
