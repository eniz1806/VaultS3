package api

import (
	"net/http"

	"github.com/eniz1806/VaultS3/internal/backup"
	"github.com/eniz1806/VaultS3/internal/metadata"
)

func (h *APIHandler) handleBackupList(w http.ResponseWriter, r *http.Request) {
	if h.backupSched == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	records, err := h.backupSched.ListRecords(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if records == nil {
		records = []metadata.BackupRecord{}
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *APIHandler) handleBackupTrigger(w http.ResponseWriter, r *http.Request) {
	if h.backupSched == nil {
		writeError(w, http.StatusBadRequest, "backup not enabled")
		return
	}

	msg := h.backupSched.TriggerBackup()
	writeJSON(w, http.StatusOK, map[string]string{"status": msg})
}

func (h *APIHandler) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	if h.backupSched == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"enabled": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"running": h.backupSched.IsRunning(),
		"targets": len(h.cfg.Backup.Targets),
	})
}

// SetBackupScheduler sets the backup scheduler reference.
func (h *APIHandler) SetBackupScheduler(sched *backup.Scheduler) {
	h.backupSched = sched
}
