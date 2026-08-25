package gate

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type MotorCommand struct {
	GateID    string
	Operation model.Operation
	Epoch     uint64
	Target    model.GatePosition
	Opening   int
	IssuedAt  time.Time
}

type MotorResult struct {
	GateID    string
	Operation model.Operation
	Epoch     uint64
	Position  model.GatePosition
	Opening   int
	Failure   string
	Finished  time.Time
}

type Executor interface {
	Execute(context.Context, MotorCommand) MotorResult
}

type SimulatedExecutor struct {
	Delay    time.Duration
	Failures map[string]string
}

func (executor *SimulatedExecutor) Execute(ctx context.Context, command MotorCommand) MotorResult {
	delay := executor.Delay
	if delay <= 0 {
		delay = time.Millisecond
	}
	result := MotorResult{
		GateID:    command.GateID,
		Operation: command.Operation,
		Epoch:     command.Epoch,
		Position:  command.Target,
		Opening:   command.Opening,
	}
	select {
	case <-ctx.Done():
		result.Position = model.GateFailed
		result.Failure = ctx.Err().Error()
	case <-time.After(delay):
		if failure := executor.Failures[command.GateID]; failure != "" {
			result.Position = model.GateFailed
			result.Failure = failure
		}
	}
	result.Finished = time.Now().UTC()
	return result
}

type Controller struct {
	mu       sync.Mutex
	executor Executor
	cancels  map[string]context.CancelFunc
	wait     sync.WaitGroup
}

func NewController(executor Executor) (*Controller, error) {
	if executor == nil {
		return nil, errors.New("gate executor is required")
	}
	return &Controller{executor: executor, cancels: make(map[string]context.CancelFunc)}, nil
}

func (controller *Controller) Execute(parent context.Context, command MotorCommand) <-chan MotorResult {
	result := make(chan MotorResult, 1)
	ctx, cancel := context.WithCancel(parent)
	key := workerKey(command)
	controller.mu.Lock()
	if previous := controller.cancels[key]; previous != nil {
		previous()
	}
	controller.cancels[key] = cancel
	controller.wait.Add(1)
	controller.mu.Unlock()
	go func() {
		defer controller.wait.Done()
		defer close(result)
		completed := controller.executor.Execute(ctx, command)
		result <- completed
		controller.mu.Lock()
		delete(controller.cancels, key)
		controller.mu.Unlock()
	}()
	return result
}

func (controller *Controller) CancelBefore(generation uint64) int {
	controller.mu.Lock()
	cancelled := 0
	for key, cancel := range controller.cancels {
		var workerGeneration uint64
		_, _ = fmtSscan(key, &workerGeneration)
		if workerGeneration < generation {
			cancel()
			delete(controller.cancels, key)
			cancelled++
		}
	}
	controller.mu.Unlock()
	controller.wait.Wait()
	return cancelled
}

func (controller *Controller) Wait() {
	controller.wait.Wait()
}

func workerKey(command MotorCommand) string {
	return command.Operation.ID.String() + "/" + formatUint(command.Operation.Generation) + "/" + command.GateID
}

func formatUint(value uint64) string {
	if value == 0 {
		return "0"
	}
	buffer := make([]byte, 0, 20)
	for value > 0 {
		buffer = append(buffer, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(buffer)-1; left < right; left, right = left+1, right-1 {
		buffer[left], buffer[right] = buffer[right], buffer[left]
	}
	return string(buffer)
}

func fmtSscan(key string, target *uint64) (int, error) {
	parts := []byte(key)
	slashes := 0
	value := uint64(0)
	seen := false
	for _, current := range parts {
		if current == '/' {
			slashes++
			continue
		}
		if slashes == 1 && current >= '0' && current <= '9' {
			value = value*10 + uint64(current-'0')
			seen = true
		}
		if slashes > 1 {
			break
		}
	}
	if !seen {
		return 0, errors.New("worker key has no generation")
	}
	*target = value
	return 1, nil
}
