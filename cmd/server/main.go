package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ibreez3/ai-reader/config"
	"github.com/ibreez3/ai-reader/novel"
	"github.com/ibreez3/ai-reader/service"
)

type GenerateReq struct {
	Topic       string   `json:"topic"`
	Chapters    int      `json:"chapters"`
	Words       int      `json:"words"`
	Preset      string   `json:"preset"`
	Instruction string   `json:"instruction"`
	Model       string   `json:"model"`
	System      string   `json:"system"`
	SourceText  string   `json:"source_text"`
	SourcePath  string   `json:"source_path"`
	Gender      string   `json:"gender"`
	Categories  []string `json:"categories"`
	Tags        []string `json:"tags"`
}

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	mgr := service.NewManager()

	r := gin.Default()

	r.POST("/api/generate", func(c *gin.Context) {
		var req GenerateReq
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		spec := novel.Spec{Topic: req.Topic, Chapters: req.Chapters, Words: req.Words, Preset: req.Preset, Instruction: req.Instruction, Language: "zh", Model: req.Model, System: req.System, Gender: req.Gender, Categories: req.Categories, Tags: req.Tags}
		if spec.System == "" && (len(spec.Categories) > 0 || len(spec.Tags) > 0 || spec.Gender != "") {
			spec.System = novel.BuildSystemFromCategories(spec.Gender, spec.Categories, spec.Tags)
		}
		var job *service.Job
		if req.SourceText != "" || req.SourcePath != "" {
			src := req.SourceText
			if src == "" && req.SourcePath != "" {
				b, e := os.ReadFile(req.SourcePath)
				if e != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": e.Error()})
					return
				}
				src = string(b)
			}
			j, e := mgr.StartFromSource(cfg, spec, src)
			if e != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			job = j
		} else {
			j, e := mgr.Start(cfg, spec)
			if e != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": e.Error()})
				return
			}
			job = j
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": job.ID})
	})

	r.GET("/api/categories", func(c *gin.Context) {
		c.JSON(http.StatusOK, service.GetCategories())
	})

	r.GET("/api/progress", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
			return
		}
		j := mgr.Get(id)
		if j == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": j.Status, "completed": j.Completed, "total": j.Total, "dir": j.Dir, "error": j.Error, "log": j.LogPath})
	})

	r.GET("/api/result", func(c *gin.Context) {
		id := c.Query("id")
		j := mgr.Get(id)
		if j == nil || j.Status != service.JobDone {
			c.JSON(http.StatusNotFound, gin.H{"error": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"dir": j.Dir, "log": j.LogPath})
	})

	r.GET("/api/log", func(c *gin.Context) {
		id := c.Query("id")
		j := mgr.Get(id)
		if j == nil || j.LogPath == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		b, err := os.ReadFile(j.LogPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "text/plain; charset=utf-8", b)
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
