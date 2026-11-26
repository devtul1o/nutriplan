package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Food: valores nutricionais por 100 g.
type Food struct {
	Source   string  `json:"source"`
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Kcal     float64 `json:"kcal"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	Fiber    float64 `json:"fiber"`
}

func fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

func (s *server) loadTaco() error {
	var raw []struct {
		ID       int     `json:"id"`
		Name     string  `json:"name"`
		Category string  `json:"category"`
		Kcal     float64 `json:"kcal"`
		Protein  float64 `json:"protein"`
		Fat      float64 `json:"fat"`
		Carbs    float64 `json:"carbs"`
		Fiber    float64 `json:"fiber"`
	}
	if err := json.Unmarshal(tacoRaw, &raw); err != nil {
		return err
	}
	for _, r := range raw {
		s.taco = append(s.taco, Food{"taco", strconv.Itoa(r.ID), r.Name, r.Category, r.Kcal, r.Protein, r.Carbs, r.Fat, r.Fiber})
		s.foldNames = append(s.foldNames, fold(r.Name))
	}
	return nil
}

func (s *server) searchTaco(q string) []Food {
	tokens := strings.Fields(fold(q))
	type hit struct {
		f     Food
		score int
	}
	var hits []hit
	for i, n := range s.foldNames {
		ok := true
		for _, t := range tokens {
			if !strings.Contains(n, t) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		score := len(n)
		if strings.HasPrefix(n, tokens[0]) {
			score -= 1000
		}
		hits = append(hits, hit{s.taco[i], score})
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].score < hits[b].score })
	if len(hits) > 25 {
		hits = hits[:25]
	}
	out := make([]Food, len(hits))
	for i, h := range hits {
		out[i] = h.f
	}
	return out
}

var usdaClient = &http.Client{Timeout: 8 * time.Second}

func (s *server) searchUSDA(q string) ([]Food, int, string) {
	if s.usdaKey == "" {
		return nil, 503, "busca USDA não configurada no servidor"
	}
	key := "usda:" + fold(q)
	if v, ok := s.cache.get(key); ok {
		return v, 200, ""
	}
	params := url.Values{}
	params.Set("api_key", s.usdaKey)
	params.Set("query", q)
	params.Set("pageSize", "10") // reduzido para traduzir rápido em paralelo
	params.Set("dataType", "Foundation,SR Legacy")
	resp, err := usdaClient.Get("https://api.nal.usda.gov/fdc/v1/foods/search?" + params.Encode())
	if err != nil {
		return nil, 502, "falha ao consultar a USDA"
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return nil, 503, "limite de consultas da USDA atingido, tente em breve"
	}
	if resp.StatusCode != 200 {
		return nil, 502, "falha ao consultar a USDA"
	}
	var u struct {
		Foods []struct {
			FdcID         int    `json:"fdcId"`
			Description   string `json:"description"`
			FoodCategory  string `json:"foodCategory"`
			FoodNutrients []struct {
				NutrientID int     `json:"nutrientId"`
				Value      float64 `json:"value"`
			} `json:"foodNutrients"`
		} `json:"foods"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&u); err != nil {
		return nil, 502, "resposta inesperada da USDA"
	}
	out := make([]Food, 0, len(u.Foods))
	for _, f := range u.Foods {
		n := map[int]float64{}
		for _, fn := range f.FoodNutrients {
			n[fn.NutrientID] = fn.Value
		}
		kcal, ok := n[1008]
		if !ok {
			if kcal, ok = n[2047]; !ok {
				kcal = n[2048]
			}
		}
		out = append(out, Food{"usda", strconv.Itoa(f.FdcID), f.Description, f.FoodCategory,
			kcal, n[1003], n[1005], n[1004], n[1079]})
	}

	// Traduz os nomes EN→PT em paralelo antes de guardar no cache
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			out[idx].Name = translateEnToPt(out[idx].Name)
		}(i)
	}
	wg.Wait()

	s.cache.set(key, out)
	return out, 200, ""
}

func (s *server) searchFoods(w http.ResponseWriter, r *http.Request, uid int64) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(q); n < 2 || n > 80 {
		writeErr(w, 400, "digite de 2 a 80 caracteres")
		return
	}
	if !s.limiter.allow("search:"+strconv.FormatInt(uid, 10), 60, time.Minute) {
		writeErr(w, 429, "muitas buscas, aguarde um instante")
		return
	}

	// 1. Tenta na TACO primeiro
	if tacoFoods := s.searchTaco(q); len(tacoFoods) > 0 {
		writeJSON(w, 200, map[string]any{"query": q, "source": "taco", "foods": tacoFoods})
		return
	}

	// 2. Não achou na TACO: busca automaticamente na USDA (traduzindo)
	tq := translateQuery(q)
	foods, code, msg := s.searchUSDA(tq)
	if msg != "" {
		writeErr(w, code, msg)
		return
	}
	writeJSON(w, 200, map[string]any{"query": tq, "source": "usda", "foods": foods})
}
