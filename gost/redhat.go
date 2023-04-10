//go:build !scanner
// +build !scanner

package gost

import (
	"encoding/json"
	"strconv"
	"strings"

	"golang.org/x/xerrors"

	"github.com/cvelab/vuls/constant"
	"github.com/cvelab/vuls/models"
	"github.com/cvelab/vuls/util"
	gostmodels "github.com/vulsio/gost/models"
)

// RedHat is Gost client for RedHat family linux
type RedHat struct {
	Base
}

// DetectCVEs fills cve information that has in Gost
func (red RedHat) DetectCVEs(r *models.ScanResult, ignoreWillNotFix bool) (nCVEs int, err error) {
	gostRelease := r.Release
	if r.Family == constant.CentOS {
		gostRelease = strings.TrimPrefix(r.Release, "stream")
	}
	if red.driver == nil {
		prefix, err := util.URLPathJoin(red.baseURL, "redhat", major(gostRelease), "pkgs")
		if err != nil {
			return 0, xerrors.Errorf("Failed to join URLPath. err: %w", err)
		}
		responses, err := getCvesWithFixStateViaHTTP(r, prefix, "unfixed-cves")
		if err != nil {
			return 0, xerrors.Errorf("Failed to get Unfixed CVEs via HTTP. err: %w", err)
		}
		for _, res := range responses {
			// CVE-ID: RedhatCVE
			cves := map[string]gostmodels.RedhatCVE{}
			if err := json.Unmarshal([]byte(res.json), &cves); err != nil {
				return 0, xerrors.Errorf("Failed to unmarshal json. err: %w", err)
			}
			for _, cve := range cves {
				if newly := red.setUnfixedCveToScanResult(&cve, r); newly {
					nCVEs++
				}
			}
		}
	} else {
		for _, pack := range r.Packages {
			// CVE-ID: RedhatCVE
			cves, err := red.driver.GetUnfixedCvesRedhat(major(gostRelease), pack.Name, ignoreWillNotFix)
			if err != nil {
				return 0, xerrors.Errorf("Failed to get Unfixed CVEs. err: %w", err)
			}
			for _, cve := range cves {
				if newly := red.setUnfixedCveToScanResult(&cve, r); newly {
					nCVEs++
				}
			}
		}
	}
	return nCVEs, nil
}

func (red RedHat) fillCvesWithRedHatAPI(r *models.ScanResult) error {
	cveIDs := []string{}
	for cveID, vuln := range r.ScannedCves {
		if _, ok := vuln.CveContents[models.RedHatAPI]; ok {
			continue
		}
		cveIDs = append(cveIDs, cveID)
	}

	if red.driver == nil {
		prefix, err := util.URLPathJoin(red.baseURL, "redhat", "cves")
		if err != nil {
			return err
		}
		responses, err := getCvesViaHTTP(cveIDs, prefix)
		if err != nil {
			return err
		}
		for _, res := range responses {
			redCve := gostmodels.RedhatCVE{}
			if err := json.Unmarshal([]byte(res.json), &redCve); err != nil {
				return err
			}
			if redCve.ID == 0 {
				continue
			}
			red.setFixedCveToScanResult(&redCve, r)
		}
	} else {
		redCves, err := red.driver.GetRedhatMulti(cveIDs)
		if err != nil {
			return err
		}
		for _, redCve := range redCves {
			if len(redCve.Name) == 0 {
				continue
			}
			red.setFixedCveToScanResult(&redCve, r)
		}
	}

	return nil
}

func (red RedHat) setFixedCveToScanResult(cve *gostmodels.RedhatCVE, r *models.ScanResult) {
	cveCont, mitigations := red.ConvertToModel(cve)
	v, ok := r.ScannedCves[cveCont.CveID]
	if ok {
		if v.CveContents == nil {
			v.CveContents = models.NewCveContents(*cveCont)
		} else {
			v.CveContents[models.RedHatAPI] = []models.CveContent{*cveCont}
		}
	} else {
		v = models.VulnInfo{
			CveID:       cveCont.CveID,
			CveContents: models.NewCveContents(*cveCont),
			Confidences: models.Confidences{models.RedHatAPIMatch},
		}
	}
	v.Mitigations = append(v.Mitigations, mitigations...)
	r.ScannedCves[cveCont.CveID] = v
}

func (red RedHat) setUnfixedCveToScanResult(cve *gostmodels.RedhatCVE, r *models.ScanResult) (newly bool) {
	cveCont, mitigations := red.ConvertToModel(cve)
	v, ok := r.ScannedCves[cve.Name]
	if ok {
		if v.CveContents == nil {
			v.CveContents = models.NewCveContents(*cveCont)
		} else {
			v.CveContents[models.RedHatAPI] = []models.CveContent{*cveCont}
		}
	} else {
		v = models.VulnInfo{
			CveID:       cveCont.CveID,
			CveContents: models.NewCveContents(*cveCont),
			Confidences: models.Confidences{models.RedHatAPIMatch},
		}
		newly = true
	}
	v.Mitigations = append(v.Mitigations, mitigations...)

	gostRelease := r.Release
	if r.Family == constant.CentOS {
		gostRelease = strings.TrimPrefix(r.Release, "stream")
	}
	pkgStats := red.mergePackageStates(v, cve.PackageState, r.Packages, gostRelease)
	if 0 < len(pkgStats) {
		v.AffectedPackages = pkgStats
		r.ScannedCves[cve.Name] = v
	}
	return
}

func (red RedHat) mergePackageStates(v models.VulnInfo, ps []gostmodels.RedhatPackageState, installed models.Packages, release string) (pkgStats models.PackageFixStatuses) {
	pkgStats = v.AffectedPackages
	for _, pstate := range ps {
		if pstate.Cpe !=
			"cpe:/o:redhat:enterprise_linux:"+major(release) {
			return
		}

		if !(pstate.FixState == "Will not fix" ||
			pstate.FixState == "Fix deferred" ||
			pstate.FixState == "Affected") {
			return
		}

		if _, ok := installed[pstate.PackageName]; !ok {
			return
		}

		notFixedYet := false
		switch pstate.FixState {
		case "Will not fix", "Fix deferred", "Affected":
			notFixedYet = true
		}

		pkgStats = pkgStats.Store(models.PackageFixStatus{
			Name:        pstate.PackageName,
			FixState:    pstate.FixState,
			NotFixedYet: notFixedYet,
		})
	}
	return
}

func (red RedHat) parseCwe(str string) (cwes []string) {
	if str != "" {
		s := strings.Replace(str, "(", "|", -1)
		s = strings.Replace(s, ")", "|", -1)
		s = strings.Replace(s, "->", "|", -1)
		for _, s := range strings.Split(s, "|") {
			if s != "" {
				cwes = append(cwes, s)
			}
		}
	}
	return
}

// ConvertToModel converts gost model to vuls model
func (red RedHat) ConvertToModel(cve *gostmodels.RedhatCVE) (*models.CveContent, []models.Mitigation) {
	cwes := red.parseCwe(cve.Cwe)

	details := []string{}
	for _, detail := range cve.Details {
		details = append(details, detail.Detail)
	}

	v2score := 0.0
	if cve.Cvss.CvssBaseScore != "" {
		v2score, _ = strconv.ParseFloat(cve.Cvss.CvssBaseScore, 64)
	}
	v2severity := ""
	if v2score != 0 {
		v2severity = cve.ThreatSeverity
	}

	v3score := 0.0
	if cve.Cvss3.Cvss3BaseScore != "" {
		v3score, _ = strconv.ParseFloat(cve.Cvss3.Cvss3BaseScore, 64)
	}
	v3severity := ""
	if v3score != 0 {
		v3severity = cve.ThreatSeverity
	}

	refs := []models.Reference{}
	for _, r := range cve.References {
		refs = append(refs, models.Reference{Link: r.Reference})
	}

	vendorURL := "https://access.redhat.com/security/cve/" + cve.Name
	mitigations := []models.Mitigation{}
	if cve.Mitigation != "" {
		mitigations = []models.Mitigation{
			{
				CveContentType: models.RedHatAPI,
				Mitigation:     cve.Mitigation,
				URL:            vendorURL,
			},
		}
	}

	return &models.CveContent{
		Type:          models.RedHatAPI,
		CveID:         cve.Name,
		Title:         cve.Bugzilla.Description,
		Summary:       strings.Join(details, "\n"),
		Cvss2Score:    v2score,
		Cvss2Vector:   cve.Cvss.CvssScoringVector,
		Cvss2Severity: v2severity,
		Cvss3Score:    v3score,
		Cvss3Vector:   cve.Cvss3.Cvss3ScoringVector,
		Cvss3Severity: v3severity,
		References:    refs,
		CweIDs:        cwes,
		Published:     cve.PublicDate,
		SourceLink:    vendorURL,
	}, mitigations
}
