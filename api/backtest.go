package api

import (
	"fmt"
	"net/http"
	"aetheris/backtest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StartBacktestRequest request body for starting a backtest
type StartBacktestRequest struct {
	Strategy       string  `json:"strategy" binding:"required"`
	Symbol         string  `json:"symbol" binding:"required"`
	Timeframe      string  `json:"timeframe" binding:"required"` // e.g. "15m"
	InitialBalance float64 `json:"initial_balance" binding:"required"`
	Leverage       int     `json:"leverage" binding:"required"`
	Description    string  `json:"description"`
}

// handleStartBacktest starts a new backtest simulation
func (s *Server) handleStartBacktest(c *gin.Context) {
	var req StartBacktestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := backtest.RunConfig{
		ID:             uuid.New().String(),
		Strategy:       req.Strategy,
		Symbol:         req.Symbol,
		Timeframe:      req.Timeframe,
		InitialBalance: req.InitialBalance,
		Leverage:       req.Leverage,
		Description:    req.Description,
		Status:         "pending",
		StartTime:      time.Now().Unix(), // This is run start time, not sim start time
	}

	// Determine data file path (Hardcoded for beta MVP, should be dynamic later)
	// We assume we have a data file for the symbol+timeframe.
	// For now, let's look for `data/{symbol}_{timeframe}.json` or use a default one.
	dataFilePath := fmt.Sprintf("data/%s_%s.json", req.Symbol, req.Timeframe)

	engine, err := s.backtestManager.StartRun(config, dataFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start backtest: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Backtest started",
		"id":      engine.Config.ID,
	})
}

// handleListBacktestRuns lists all backtest runs
func (s *Server) handleListBacktestRuns(c *gin.Context) {
	runs, err := s.backtestManager.ListRuns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runs"})
		return
	}
	c.JSON(http.StatusOK, runs)
}

// handleGetBacktestRun gets details of a specific run
func (s *Server) handleGetBacktestRun(c *gin.Context) {
	id := c.Param("id")
	result, err := s.backtestManager.GetRun(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Run not found"})
		return
	}
	c.JSON(http.StatusOK, result)
}
