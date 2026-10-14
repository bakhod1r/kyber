package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/domain/repotest"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/memory"
)

func TestContract(t *testing.T) {
	r := memory.NewRepository()
	repotest.Run(t, r, r.Messages())
}

func TestSavedContract(t *testing.T) {
	r := memory.NewRepository()
	repotest.RunSaved(t, r, r.Messages(), memory.NewSaved(r))
}

func TestStatusContract(t *testing.T) { repotest.RunStatuses(t, memory.NewStatuses()) }

func TestScheduledContract(t *testing.T) {
	repotest.RunScheduled(t, memory.NewRepository(), memory.NewScheduled(), memory.NewReminders())
}

func TestReceiptsContract(t *testing.T) { repotest.RunReceipts(t, memory.NewRepository()) }

func TestManagementContract(t *testing.T) { repotest.RunManagement(t, memory.NewRepository()) }

func TestGroupsContract(t *testing.T) { repotest.RunGroups(t, memory.NewGroups()) }

func TestPrefsContract(t *testing.T) { repotest.RunPrefs(t, memory.NewRepository(), memory.NewPrefs()) }

func TestBookmarksContract(t *testing.T) {
	repotest.RunBookmarks(t, memory.NewRepository(), memory.NewBookmarks())
}

func TestWorkflowsContract(t *testing.T) {
	repotest.RunWorkflows(t, memory.NewRepository(), memory.NewWorkflows())
}

func TestWebhooksContract(t *testing.T) {
	repotest.RunWebhooks(t, memory.NewRepository(), memory.NewWebhooks())
}

func TestBotsContract(t *testing.T) { repotest.RunBots(t, memory.NewBots()) }

func TestSectionOrdersContract(t *testing.T) { repotest.RunSectionOrders(t, memory.NewSectionOrders()) }
