package myks

import (
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type Helm struct {
	ident    string
	app      *Application
	additive bool
}

func (h *Helm) IsAdditive() bool {
	return h.additive
}

func (h *Helm) Ident() string {
	return h.ident
}

func (h *Helm) Render(_ string) (string, error) {
	chartDir, err := h.app.getVendoredDir(h.app.e.g.HelmChartsDirName)
	if err != nil {
		log.Err(err).Str("app", h.app.Name).Msg("Unable to get helm charts dir")
		return "", err
	}
	chartDirs := getSubDirs(chartDir)
	if len(chartDirs) == 0 {
		log.Debug().Str("app", h.app.Name).Msg("No charts to process")
		return "", nil
	}

	helmConfig, err := h.getHelmConfig()
	if err != nil {
		log.Warn().Err(err).Str("app", h.app.Name).Msg("Unable to get helm config")
		return "", err
	}

	var commonHelmArgs []string

	// FIXME: move Namespace to a per-chart config
	if helmConfig.Namespace == "" {
		helmConfig.Namespace = h.app.e.g.NamespacePrefix + h.app.Name
	}
	commonHelmArgs = append(commonHelmArgs, "--namespace", helmConfig.Namespace)

	if helmConfig.KubeVersion != "" {
		commonHelmArgs = append(commonHelmArgs, "--kube-version", helmConfig.KubeVersion)
	}

	// FIXME: move IncludeCRDs to a per-chart config
	if helmConfig.IncludeCRDs {
		commonHelmArgs = append(commonHelmArgs, "--include-crds")
	}

	var outputs []string

	for _, chartDir := range chartDirs {
		chartName := filepath.Base(chartDir)
		var helmValuesFile string
		if helmValuesFile, err = h.app.prepareValuesFile("helm", chartName); err != nil {
			log.Warn().Err(err).Str("app", h.app.Name).Msg("Unable to prepare helm values")
			return "", err
		}

		// FIXME: replace h.app.Name with a name of the chart being processed
		helmArgs := []string{
			"template",
			"--skip-tests",
			chartName,
			chartDir,
		}

		if helmValuesFile != "" {
			helmArgs = append(helmArgs, "--values", helmValuesFile)
		}

		res, err := runCmd("helm", nil, append(helmArgs, commonHelmArgs...))
		if err != nil {
			log.Warn().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to run helm")
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("app", h.app.Name).Str("chart", chartName).Msg("No helm output")
			continue
		}

		outputs = append(outputs, res.Stdout)

	}

	return strings.Join(outputs, "---\n"), nil
}

func (h *Helm) getHelmConfig() (HelmConfig, error) {
	dataValuesYaml, err := h.app.e.g.ytt(h.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		log.Warn().Err(err).Str("app", h.app.Name).Msg("Unable to inspect data values")
		return HelmConfig{}, err
	}

	var helmConfig struct {
		Helm HelmConfig
	}
	err = yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &helmConfig)
	if err != nil {
		log.Warn().Err(err).Str("app", h.app.Name).Msg("Unable to unmarshal data values")
		return HelmConfig{}, err
	}

	return helmConfig.Helm, nil
}
