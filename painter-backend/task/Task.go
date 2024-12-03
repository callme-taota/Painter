package task

import (
	"time"

	"github.com/callme-taota/painter/painter-backend/common"
	"github.com/callme-taota/painter/painter-backend/conf"
	"github.com/callme-taota/tolog"
)

func init() {
	mgr := newTaskMgr()
	mgr.registerTask(newUserLastLoginUpdateTask())
	mgr.registerTask(newVisitorTask())
	common.Register(mgr)
}

type task interface {
	taskFn() func()
	taskName() string
	duration() time.Duration
	withValue(args ...interface{}) error
}

type taskManager struct {
	taskList []task
	timeZone string
}

func newTaskMgr() *taskManager {
	return &taskManager{taskList: make([]task, 0)}
}

func (t *taskManager) Name() string {
	return common.TASK_MODULE
}

func (t *taskManager) Register(conf conf.Config) (common.Module, error) {
	t.timeZone = conf.Server.Timezone
	return t, nil
}

func (t *taskManager) Start() error {
	time.Sleep(time.Second * 5)
	for _, tsk := range t.taskList {
		tempTask := tsk
		if tempTask.taskName() == "visitorTask" {
			tempTask.withValue(t.timeZone)
		}
		go func() {
			tolog.Infof("Start task: %s", tempTask).PrintAndWriteSafe()
			TickTasks(tempTask.taskFn(), tempTask.duration())
		}()
	}
	return nil
}

func TickTasks(taskFn func(), duration time.Duration) {
	tick := time.NewTimer(duration)
	defer tick.Stop()

	start := time.Now()
	for {
		taskFn()

		cost := time.Since(start)
		ttl := duration - cost
		if ttl < 0 {
			ttl = 0
		}
		tick.Reset(ttl)
		select {
		case <-tick.C:
		}
		start = time.Now()
	}
}

func (t *taskManager) registerTask(tsk task) {
	t.taskList = append(t.taskList, tsk)
}
