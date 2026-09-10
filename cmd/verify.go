package cmd

import (
	"fmt"
	"time"

	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	verifySkipSec  bool
	verifySkipVuln bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Pipeline completo de verificación local y CI (lint, sec, vulncheck, tests)",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()
		ui.Header("VERIFICACIÓN COMPLETA DE CÓDIGO (godev verify)")

		// 1. Lint
		ui.Step("Paso 1/4: Análisis de estilo y bugs (golangci-lint)")
		if err := lintCmd.RunE(cmd, nil); err != nil {
			return err
		}

		// 2. Sec
		if !verifySkipSec {
			ui.Step("Paso 2/4: Análisis estático de seguridad (gosec)")
			if err := secCmd.RunE(cmd, nil); err != nil {
				return err
			}
		} else {
			ui.Warn("Paso 2/4: gosec omitido (--skip-sec)")
		}

		// 3. Vulncheck
		if !verifySkipVuln {
			ui.Step("Paso 3/4: Comprobación de CVEs en dependencias (govulncheck)")
			if err := vulncheckCmd.RunE(cmd, nil); err != nil {
				return err
			}
		} else {
			ui.Warn("Paso 3/4: govulncheck omitido (--skip-vuln)")
		}

		// 4. Test con Race
		ui.Step("Paso 4/4: Tests unitarios con -race y -shuffle=on")
		if err := testCmd.RunE(cmd, nil); err != nil {
			return err
		}

		ui.Header(fmt.Sprintf("✅ PROYECTO VERIFICADO AL 100%% EN %s", time.Since(start).Round(time.Millisecond)))
		return nil
	},
}

func init() {
	verifyCmd.Flags().BoolVar(&verifySkipSec, "skip-sec", false, "Omitir análisis gosec")
	verifyCmd.Flags().BoolVar(&verifySkipVuln, "skip-vuln", false, "Omitir comprobación govulncheck")
	rootCmd.AddCommand(verifyCmd)
}
