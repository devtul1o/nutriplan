package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Dicionário PT→EN para buscar na USDA (que só entende inglês).
// Chaves sem acento e em minúsculas. Valor "" = palavra descartada.
var ptEn = map[string]string{
	// descartadas
	"de": "", "da": "", "do": "", "das": "", "dos": "", "com": "", "e": "", "em": "", "ao": "", "a": "", "o": "", "na": "", "no": "",
	// proteínas
	"frango": "chicken", "peito": "breast", "coxa": "thigh", "sobrecoxa": "thigh", "asa": "wing", "carne": "beef", "bovina": "beef", "boi": "beef",
	"porco": "pork", "suina": "pork", "lombo": "loin", "costela": "rib", "peixe": "fish", "salmao": "salmon", "atum": "tuna", "tilapia": "tilapia",
	"sardinha": "sardine", "bacalhau": "cod", "camarao": "shrimp", "ovo": "egg", "clara": "white", "gema": "yolk", "figado": "liver",
	"presunto": "ham", "peru": "turkey", "linguica": "sausage", "salsicha": "frankfurter", "hamburguer": "hamburger", "tofu": "tofu", "pato": "duck", "cordeiro": "lamb",
	// laticínios
	"leite": "milk", "queijo": "cheese", "iogurte": "yogurt", "manteiga": "butter", "requeijao": "cream cheese", "ricota": "ricotta",
	"mussarela": "mozzarella", "creme": "cream", "desnatado": "nonfat", "integral": "whole", "magro": "lean",
	// cereais e tubérculos
	"arroz": "rice", "feijao": "beans", "pao": "bread", "macarrao": "pasta", "massa": "pasta", "aveia": "oats", "milho": "corn", "trigo": "wheat",
	"farinha": "flour", "batata": "potato", "doce": "sweet", "mandioca": "cassava", "aipim": "cassava", "macaxeira": "cassava", "inhame": "yam",
	"tapioca": "tapioca", "quinoa": "quinoa", "granola": "granola", "cereal": "cereal", "biscoito": "cookie", "bolo": "cake",
	// leguminosas e oleaginosas
	"lentilha": "lentils", "ervilha": "peas", "grao-de-bico": "chickpeas", "soja": "soybeans", "amendoim": "peanuts", "castanha": "nuts",
	"nozes": "walnuts", "amendoa": "almonds", "coco": "coconut",
	// frutas
	"banana": "banana", "maca": "apple", "laranja": "orange", "limao": "lemon", "mamao": "papaya", "abacate": "avocado", "abacaxi": "pineapple",
	"melancia": "watermelon", "uva": "grapes", "morango": "strawberries", "manga": "mango", "melao": "melon", "pera": "pear", "pessego": "peach",
	"ameixa": "plum", "kiwi": "kiwi", "goiaba": "guava", "maracuja": "passion fruit", "fruta": "fruit",
	// vegetais
	"tomate": "tomato", "alface": "lettuce", "cenoura": "carrots", "brocolis": "broccoli", "couve": "kale", "espinafre": "spinach", "cebola": "onion",
	"alho": "garlic", "pepino": "cucumber", "abobora": "pumpkin", "abobrinha": "zucchini", "beterraba": "beets", "repolho": "cabbage",
	"couve-flor": "cauliflower", "pimentao": "bell pepper", "berinjela": "eggplant", "quiabo": "okra", "vagem": "green beans", "cogumelo": "mushrooms",
	// outros
	"azeite": "olive oil", "oleo": "oil", "acucar": "sugar", "sal": "salt", "mel": "honey", "chocolate": "chocolate", "cafe": "coffee", "cha": "tea",
	"suco": "juice", "agua": "water", "refrigerante": "soda", "cerveja": "beer", "vinho": "wine", "pizza": "pizza", "maionese": "mayonnaise",
	// preparo
	"cozido": "cooked", "cozida": "cooked", "cru": "raw", "crua": "raw", "grelhado": "grilled", "grelhada": "grilled", "frito": "fried", "frita": "fried",
	"assado": "roasted", "assada": "roasted", "refogado": "sauteed", "refogada": "sauteed", "sem": "without", "pele": "skin", "gordura": "fat",
}

var (
	transClient = &http.Client{Timeout: 4 * time.Second}
	transCache  = map[string]string{}
	transMu     sync.RWMutex
)

func translateAPI(text, from, to string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	key := from + "|" + to + ":" + strings.ToLower(text)
	transMu.RLock()
	if cached, ok := transCache[key]; ok {
		transMu.RUnlock()
		return cached
	}
	transMu.RUnlock()

	endpoint := "https://api.mymemory.translated.net/get?q=" +
		url.QueryEscape(text) + "&langpair=" + from + "|" + to

	resp, err := transClient.Get(endpoint)
	if err != nil {
		return text
	}
	defer resp.Body.Close()

	var res struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || res.ResponseData.TranslatedText == "" {
		return text
	}

	out := res.ResponseData.TranslatedText
	transMu.Lock()
	transCache[key] = out
	transMu.Unlock()
	return out
}

func translateEnToPt(text string) string {
	return translateAPI(text, "en", "pt")
}

func translateQuery(q string) string {
	tokens := strings.FieldsFunc(fold(q), func(r rune) bool { return r == ' ' || r == ',' || r == ';' })
	out := make([]string, 0, len(tokens))
	allFound := true
	for _, t := range tokens {
		en, ok := ptEn[t]
		if !ok && len(t) > 3 && strings.HasSuffix(t, "s") {
			en, ok = ptEn[t[:len(t)-1]]
		}
		if ok {
			if en != "" {
				out = append(out, en)
			}
			continue
		}
		allFound = false
		out = append(out, t)
	}
	if !allFound {
		return translateAPI(q, "pt", "en")
	}
	if len(out) == 0 {
		return strings.TrimSpace(q)
	}
	return strings.Join(out, " ")
}
