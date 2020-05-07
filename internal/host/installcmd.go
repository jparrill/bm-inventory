package host

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/jinzhu/gorm"

	"github.com/filanov/bm-inventory/internal/hardware"
	"github.com/filanov/bm-inventory/models"
)

type installCmd struct {
	baseCmd
	db                *gorm.DB
	hwValidator       hardware.Validator
	instructionConfig InstructionConfig
}

func NewInstallCmd(log logrus.FieldLogger, db *gorm.DB, hwValidator hardware.Validator, instructionConfig InstructionConfig) *installCmd {
	return &installCmd{
		baseCmd:           baseCmd{log: log},
		db:                db,
		hwValidator:       hwValidator,
		instructionConfig: instructionConfig,
	}
}

func (i *installCmd) GetStep(ctx context.Context, host *models.Host) (*models.Step, error) {
	step := &models.Step{}
	step.StepType = models.StepTypeExecute
	step.Command = "sudo podman"

	var role = host.Role
	if host.Bootstrap {
		role = RoleBootstrap
	}

	const cmdArgsTmpl = `run -v /dev:/dev:rw -v /opt:/opt:rw --privileged --pid=host  {{.INSTALLER}} --role {{.ROLE}}  --cluster-id {{.CLUSTER_ID}}  --host {{.HOST}} --port {{.PORT}} --boot-device {{.BOOT_DEVICE}}`

	t, err := template.New("cmd").Parse(cmdArgsTmpl)
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		"HOST":        i.instructionConfig.InventoryURL,
		"PORT":        i.instructionConfig.InventoryPort,
		"CLUSTER_ID":  string(host.ClusterID),
		"ROLE":        role,
		"INSTALLER":   i.instructionConfig.InstallerImage,
		"BOOT_DEVICE": "",
	}
	bootdevice, err := getBootDevice(i.log, i.hwValidator, *host)
	if err != nil {
		return nil, err
	}
	data["BOOT_DEVICE"] = bootdevice
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, data); err != nil {
		return nil, err
	}
	step.Args = strings.Split(buf.String(), " ")

	return step, nil
}

func getBootDevice(log logrus.FieldLogger, hwValidator hardware.Validator, host models.Host) (string, error) {
	disks, err := hwValidator.GetHostValidDisks(&host)
	if err != nil || len(disks) == 0 {
		err := fmt.Errorf("Failed to get valid disks on host with id %s", host.ID)
		log.WithError(err)
		return "", err
	}
	return fmt.Sprintf("/dev/%s", disks[0].Name), nil
}
