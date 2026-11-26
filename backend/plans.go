package main

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxPlans        = 3
	maxDays         = 7
	maxMeals        = 6
	maxItemsPerMeal = 60
)

type PlanItem struct {
	Source  string  `json:"source"` // "taco" ou "usda"
	FoodID  string  `json:"foodId"`
	Name    string  `json:"name"`
	Grams   float64 `json:"grams"`
	Kcal    float64 `json:"kcal"` // valores por 100 g
	Protein float64 `json:"protein"`
	Carbs   float64 `json:"carbs"`
	Fat     float64 `json:"fat"`
	Fiber   float64 `json:"fiber"`
}

type Meal struct {
	Name  string     `json:"name"`
	Items []PlanItem `json:"items"`
}

type DayPlan struct {
	Day   string `json:"day"` // "seg", "ter", "qua", "qui", "sex", "sab", "dom"
	Meals []Meal `json:"meals"`
}

type planIn struct {
	Name string    `json:"name"`
	Goal string    `json:"goal"`
	Days []DayPlan `json:"days"`
}

type planOut struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal"`
	Days      []DayPlan `json:"days"`
	UpdatedAt string    `json:"updatedAt"`
}

func okNum(v, max float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= max }

func (p *planIn) validate() string {
	p.Name = strings.TrimSpace(p.Name)
	p.Goal = strings.TrimSpace(p.Goal)
	if n := utf8.RuneCountInString(p.Name); n < 1 || n > 80 {
		return "dê um nome ao plano (até 80 caracteres)"
	}
	if utf8.RuneCountInString(p.Goal) > 200 {
		return "objetivo muito longo (máx. 200 caracteres)"
	}
	if len(p.Days) > maxDays {
		return "um plano pode ter no máximo 7 dias"
	}
	for dIdx := range p.Days {
		d := &p.Days[dIdx]
		d.Day = strings.TrimSpace(d.Day)
		if d.Day == "" || len(d.Day) > 20 {
			return "dia da semana inválido"
		}
		if len(d.Meals) > maxMeals {
			return "cada dia pode ter no máximo 6 refeições"
		}
		for i := range d.Meals {
			m := &d.Meals[i]
			m.Name = strings.TrimSpace(m.Name)
			if n := utf8.RuneCountInString(m.Name); n < 1 || n > 40 {
				return "nome de refeição inválido (até 40 caracteres)"
			}
			if len(m.Items) > maxItemsPerMeal {
				return "muitos alimentos em uma refeição"
			}
			for _, it := range m.Items {
				if it.Source != "taco" && it.Source != "usda" {
					return "fonte de alimento inválida"
				}
				if n := utf8.RuneCountInString(it.Name); n < 1 || n > 200 || len(it.FoodID) > 20 {
					return "alimento inválido"
				}
				if !okNum(it.Grams, 5000) || it.Grams <= 0 {
					return "gramatura inválida (0 a 5000 g)"
				}
				if !okNum(it.Kcal, 1000) || !okNum(it.Protein, 100) || !okNum(it.Carbs, 100) || !okNum(it.Fat, 100) || !okNum(it.Fiber, 100) {
					return "valores nutricionais inválidos"
				}
			}
			if m.Items == nil {
				m.Items = []PlanItem{}
			}
		}
		if d.Meals == nil {
			d.Meals = []Meal{}
		}
	}
	if p.Days == nil {
		p.Days = []DayPlan{}
	}
	return ""
}

func (s *server) listPlans(w http.ResponseWriter, r *http.Request, uid int64) {
	rows, err := s.db.Query(`SELECT id,name,goal,data,updated_at FROM plans WHERE user_id=? ORDER BY id`, uid)
	if err != nil {
		writeErr(w, 500, "erro interno")
		return
	}
	defer rows.Close()
	out := []planOut{}
	for rows.Next() {
		var p planOut
		var data string
		if err := rows.Scan(&p.ID, &p.Name, &p.Goal, &data, &p.UpdatedAt); err != nil {
			continue
		}
		// Tenta ler no formato novo (por dias)
		_ = json.Unmarshal([]byte(data), &p.Days)

		// Compatibilidade com planos antigos salvos só como []Meal
		if len(p.Days) > 0 && p.Days[0].Day == "" {
			var oldMeals []Meal
			if err := json.Unmarshal([]byte(data), &oldMeals); err == nil {
				p.Days = []DayPlan{{Day: "seg", Meals: oldMeals}}
			}
		}
		if p.Days == nil {
			p.Days = []DayPlan{}
		}
		out = append(out, p)
	}
	writeJSON(w, 200, out)
}

func (s *server) createPlan(w http.ResponseWriter, r *http.Request, uid int64) {
	var in planIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 400, msg)
		return
	}
	data, _ := json.Marshal(in.Days)
	res, err := s.db.Exec(`INSERT INTO plans(user_id,name,goal,data)
		SELECT ?,?,?,? WHERE (SELECT COUNT(*) FROM plans WHERE user_id=?) < ?`,
		uid, in.Name, in.Goal, string(data), uid, maxPlans)
	if err != nil {
		writeErr(w, 500, "erro interno")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 409, "você já tem 3 planos alimentares. Exclua um para criar outro")
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 201, map[string]int64{"id": id})
}

func (s *server) updatePlan(w http.ResponseWriter, r *http.Request, uid int64) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "id inválido")
		return
	}
	var in planIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 400, msg)
		return
	}
	data, _ := json.Marshal(in.Days)
	res, err := s.db.Exec(`UPDATE plans SET name=?,goal=?,data=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=?`,
		in.Name, in.Goal, string(data), id, uid)
	if err != nil {
		writeErr(w, 500, "erro interno")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "plano não encontrado")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *server) deletePlan(w http.ResponseWriter, r *http.Request, uid int64) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "id inválido")
		return
	}
	res, err := s.db.Exec(`DELETE FROM plans WHERE id=? AND user_id=?`, id, uid)
	if err != nil {
		writeErr(w, 500, "erro interno")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "plano não encontrado")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
