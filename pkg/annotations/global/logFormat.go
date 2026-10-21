package global

import (
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type LogFormat struct {
	defaults *models.Defaults
	name     string
}

func NewLogFormat(n string, d *models.Defaults) *LogFormat {
	return &LogFormat{name: n, defaults: d}
}

func (a *LogFormat) GetName() string {
	return a.name
}

func (a *LogFormat) Process(k store.K8s, annotations ...map[string]string) error {
	if common.IsSet(a.name, annotations...) {
		utils.Once("deprecation-Defaults-"+a.name, func() {
			logger.Warningf("DEPRECATED: annotation %q will be removed in a future version, use the Defaults custom resource instead", a.name)
		})
	}
	input := common.GetValue(a.GetName(), annotations...)
	if input != "" {
		input = "'" + strings.TrimSpace(input) + "'"
	}
	a.defaults.LogFormat = input
	return nil
}
