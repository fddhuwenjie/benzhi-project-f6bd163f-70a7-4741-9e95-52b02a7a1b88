package application

import (
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
	"encoding/json"
	"fmt"
)

func (s *Service) Review(command ReviewCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "case.reviewed", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		if len(command.Checklist) == 0 {
			return domain.NewError(domain.CodeValidation, "复核清单不能为空")
		}
		return c.Review(command.Decision, command.ActorID, command.ReviewNote, command.Checklist, s.now())
	})
}

func (s *Service) GetCase(caseID string) (*domain.DrillCase, error) {
	c, err := s.repo.Load(caseID)
	if err == nil {
		c.DecorateTemporal(s.now())
	}
	return c, err
}

func (s *Service) ListCases() ([]*domain.DrillCase, error) {
	cases, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for _, c := range cases {
		c.DecorateTemporal(s.now())
	}
	return cases, nil
}

func (s *Service) DownloadDossier(caseID string) ([]byte, string, error) {
	c, err := s.repo.Load(caseID)
	if err != nil {
		return nil, "", err
	}
	if c.Status != domain.StatusApproved || c.Dossier == nil {
		return nil, "", domain.NewError(domain.CodeState, "仅已批准案件可以下载就绪档案")
	}
	valid, _, err := domain.VerifyDossier(c.Dossier)
	if err != nil {
		return nil, "", err
	}
	if !valid {
		return nil, "", domain.NewError(domain.CodeIntegrity, "就绪档案摘要校验失败")
	}
	payload, err := json.Marshal(DossierDownload{Schema: "readiness-dossier-package/v1", CaseID: c.CaseID, SealedAt: c.Dossier.SealedAt.UTC(), Manifest: c.Dossier.Manifest, ContentDigest: c.Dossier.ContentDigest})
	if err != nil {
		return nil, "", fmt.Errorf("生成下载包: %w", err)
	}
	return payload, safeDownloadName(c.CaseID), nil
}

func safeDownloadName(caseID string) string {
	out := make([]rune, 0, len(caseID))
	for _, r := range caseID {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "case"
	}
	return string(out) + "-readiness-dossier.json"
}

type DossierVerification struct {
	Valid          bool   `json:"valid"`
	StoredDigest   string `json:"stored_digest"`
	ComputedDigest string `json:"computed_digest"`
}

type dossierVerificationCache struct {
	revision int64
	result   DossierVerification
}

func (s *Service) VerifyDossier(caseID string) (*DossierVerification, error) {
	c, err := s.repo.Load(caseID)
	if err != nil {
		return nil, err
	}
	s.verificationMu.RLock()
	cached, ok := s.verificationCache[caseID]
	s.verificationMu.RUnlock()
	if ok && cached.revision == c.Revision {
		result := cached.result
		return &result, nil
	}
	valid, computed, err := domain.VerifyDossier(c.Dossier)
	if err != nil {
		return nil, err
	}
	result := DossierVerification{Valid: valid, StoredDigest: c.Dossier.ContentDigest, ComputedDigest: computed}
	s.verificationMu.Lock()
	s.verificationCache[caseID] = dossierVerificationCache{revision: c.Revision, result: result}
	s.verificationMu.Unlock()
	return &result, nil
}

func (s *Service) StorageIntegrity() store.Integrity {
	return s.repo.Integrity()
}

func (s *Service) GetAuditTrail(caseID string) ([]store.EventFrame, error) {
	if _, err := s.repo.Load(caseID); err != nil {
		return nil, err
	}
	return s.repo.Events(caseID)
}
