package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/application"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
	webapp "benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if cfg.SelfCheck {
		tempDir, err := os.MkdirTemp("", "benzhi-drill-selfcheck-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
		cfg.DataDir = tempDir
		return runSelfCheck(cfg)
	}
	repo, server, err := assemble(cfg)
	if err != nil {
		return err
	}
	defer repo.Close()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.Address, err)
	}
	log.Printf("化学品泄漏演练就绪门禁已启动: http://%s", cfg.Address)
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func assemble(cfg config) (*store.Repository, *http.Server, error) {
	if err := os.MkdirAll(filepath.Clean(cfg.DataDir), 0o700); err != nil {
		return nil, nil, err
	}
	repo, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}
	if integrity := repo.Integrity(); !integrity.Healthy {
		_ = repo.Close()
		return nil, nil, fmt.Errorf("事件日志完整性检查失败: %s", integrity.ErrorMessage)
	}
	service := application.NewService(repo)
	handler := webapp.NewServer(service).Handler()
	server := &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: 4 * time.Second, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: 45 * time.Second, MaxHeaderBytes: 1 << 20}
	return repo, server, nil
}

type selfCheckClient struct {
	baseURL        string
	client         *http.Client
	revision       int64
	caseID         string
	requestCounter int
}

func runSelfCheck(cfg config) error {
	repo, server, err := assemble(cfg)
	if err != nil {
		return err
	}
	defer repo.Close()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", cfg.Address, err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	client := &selfCheckClient{baseURL: "http://" + cfg.Address, client: &http.Client{Timeout: 5 * time.Second}}
	workflowErr := client.runWorkflow()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	shutdownErr := server.Shutdown(shutdownCtx)
	cancel()
	serveErr := <-done
	if workflowErr != nil {
		return workflowErr
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	log.Printf("自检通过：失败整改、定向复演、独立批准和 SHA-256 校验均完成")
	return nil
}

func (c *selfCheckClient) runWorkflow() error {
	created, err := c.post("/api/cases", map[string]any{"request_id": c.nextRequest(), "expected_revision": 0, "actor_id": "coord-selfcheck", "title": "自检泄漏演练", "building": "实验楼 A", "coordinator_id": "coord-selfcheck", "observer_ids": []string{"observer-selfcheck"}})
	if err != nil {
		return err
	}
	c.caseID, c.revision = created.CaseID, created.Revision
	baseline := domain.BaselineInput{ChemicalName: "丙酮", HazardClass: "易燃液体", AffectedZones: []string{"A-201"}, RequiredRoles: []string{"警戒", "疏散"}, ObservationPoints: []string{"alarm_time", "evacuation_rate"}, Thresholds: []domain.Threshold{{PointID: "alarm_time", Label: "报警用时", Unit: "秒", Rule: "lte", Target: 60}, {PointID: "evacuation_rate", Label: "疏散完成率", Unit: "%", Rule: "gte", Target: 100}}}
	if _, err := c.action("/baseline/freeze", "coord-selfcheck", map[string]any{"baseline": baseline}); err != nil {
		return err
	}
	for _, item := range domain.RequiredCheckItems {
		if _, err := c.action("/preflight", "coord-selfcheck", map[string]any{"item": item, "passed": true, "evidence_summary": "自检证据-" + item, "valid_until": time.Now().UTC().Add(24 * time.Hour)}); err != nil {
			return err
		}
	}
	if _, err := c.action("/sessions/start", "observer-selfcheck", map[string]any{"kind": "initial"}); err != nil {
		return err
	}
	if _, err := c.action("/sessions/events", "observer-selfcheck", map[string]any{"sequence": 1, "action": "discovery", "note": "发现模拟泄漏"}); err != nil {
		return err
	}
	if _, err := c.action("/sessions/observations", "observer-selfcheck", map[string]any{"point_id": "alarm_time", "value": 75, "evidence_summary": "计时记录"}); err != nil {
		return err
	}
	if _, err := c.action("/sessions/observations", "observer-selfcheck", map[string]any{"point_id": "evacuation_rate", "value": 100, "evidence_summary": "清点记录"}); err != nil {
		return err
	}
	finished, err := c.action("/sessions/finish", "observer-selfcheck", nil)
	if err != nil {
		return err
	}
	if finished.Status != domain.StatusRemediation || len(finished.Deviations) != 1 {
		return fmt.Errorf("首演未产生预期偏差")
	}
	deviationID := finished.Deviations[0].DeviationID
	if _, err := c.action("/deviations/remediate-batch", "coord-selfcheck", map[string]any{"items": []map[string]any{{"deviation_id": deviationID, "cause": "报警口令不熟练", "corrective_action": "完成报警口令专项训练", "evidence_digest": "training-evidence"}}, "owner_id": "owner-selfcheck", "due_at": time.Now().UTC().Add(24 * time.Hour)}); err != nil {
		return err
	}
	if _, err := c.action("/sessions/start", "observer-selfcheck", map[string]any{"kind": "retest"}); err != nil {
		return err
	}
	if _, err := c.action("/sessions/observations", "observer-selfcheck", map[string]any{"point_id": "alarm_time", "value": 45, "evidence_summary": "复演计时记录"}); err != nil {
		return err
	}
	if _, err := c.action("/sessions/finish", "observer-selfcheck", nil); err != nil {
		return err
	}
	checklist := []map[string]any{}
	for _, item := range domain.RequiredReviewChecklist {
		checklist = append(checklist, map[string]any{"item": item, "passed": true, "note": "自检确认"})
	}
	approved, err := c.action("/review", "reviewer-selfcheck", map[string]any{"decision": "approve", "review_note": "时间线完整，整改与定向复演证据充分", "checklist": checklist})
	if err != nil {
		return err
	}
	if approved.Status != domain.StatusApproved || approved.Dossier == nil {
		return fmt.Errorf("案件未进入批准终态")
	}
	var verification application.DossierVerification
	if err := c.get("/api/cases/"+c.caseID+"/dossier/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.ComputedDigest != approved.Dossier.ContentDigest {
		return fmt.Errorf("就绪档案摘要校验失败")
	}
	response, err := c.client.Get(c.baseURL + "/api/cases/" + c.caseID + "/dossier/download")
	if err != nil {
		return err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Disposition") == "" {
		return fmt.Errorf("就绪档案下载失败")
	}
	return nil
}

func (c *selfCheckClient) action(suffix, actor string, fields map[string]any) (*domain.DrillCase, error) {
	body := map[string]any{"request_id": c.nextRequest(), "expected_revision": c.revision, "actor_id": actor}
	for key, value := range fields {
		body[key] = value
	}
	result, err := c.post("/api/cases/"+c.caseID+suffix, body)
	if err == nil {
		c.revision = result.Revision
	}
	return result, err
}

func (c *selfCheckClient) nextRequest() string {
	c.requestCounter++
	return fmt.Sprintf("selfcheck-%02d", c.requestCounter)
}

func (c *selfCheckClient) post(path string, value any) (*domain.DrillCase, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data  *domain.DrillCase `json:"data"`
		Error *domain.Error     `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("解析自检响应: %w", err)
	}
	if response.StatusCode >= 300 || envelope.Error != nil {
		return nil, fmt.Errorf("自检请求 %s 失败 (%d): %v", path, response.StatusCode, envelope.Error)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("自检请求 %s 未返回案件", path)
	}
	return envelope.Data, nil
}

func (c *selfCheckClient) get(path string, target any) error {
	response, err := c.client.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error *domain.Error   `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope); err != nil {
		return err
	}
	if response.StatusCode >= 300 || envelope.Error != nil {
		return fmt.Errorf("自检查询失败 (%d): %v", response.StatusCode, envelope.Error)
	}
	return json.Unmarshal(envelope.Data, target)
}
