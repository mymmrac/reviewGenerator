package reviewer

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"
)

type ReviewsCore struct {
	InBetweenText map[string][]string `json:"in_between_text"`
	Categories    []Category          `json:"categories"`
	Features      []Feature           `json:"features"`
	random        *rand.Rand
}

func NewReviewCore(fileName string) (*ReviewsCore, error) {
	jsonFile, err := os.Open(fileName)
	defer func() {
		if err := jsonFile.Close(); err != nil {
			panic(err)
		}
	}()
	if err != nil {
		return nil, err
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var reviewCore ReviewsCore

	err = json.Unmarshal(byteValue, &reviewCore)
	if err != nil {
		return nil, err
	}

	for i := range reviewCore.Features {
		reviewCore.Features[i].EnabledFields = make(map[Field]bool)
	}

	reviewCore.random = rand.New(rand.NewSource(time.Now().UnixNano()))

	return &reviewCore, nil
}

func (rc *ReviewsCore) GetInBetweenText(name string) (string, error) {
	if texts, ok := rc.InBetweenText[name]; ok {
		return texts[rc.random.Intn(len(texts))], nil
	}
	return "", fmt.Errorf("in between text %s not found", name)
}

func (rc *ReviewsCore) HasCategory(category Category) bool {
	found := false
	for _, c := range rc.Categories {
		if c.Name == category.Name {
			found = true
			break
		}
	}
	return found
}

func (rc *ReviewsCore) HasFeature(feature Feature) bool {
	found := false
	for _, f := range rc.Features {
		if f.Name == feature.Name {
			found = true
			break
		}
	}
	return found
}

func (rc *ReviewsCore) GetCategoryWithName(name string) *Category {
	for _, c := range rc.Categories {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func (rc *ReviewsCore) GetFeatureWithName(name string) *Feature {
	for _, f := range rc.Features {
		if f.Name == name {
			return &f
		}
	}
	return nil
}

type Category struct {
	Name          string        `json:"name"`
	SubCategories []SubCategory `json:"sub_categories"`
}

type SubCategory string

func (sc SubCategory) String() string {
	return string(sc)
}
func (c *Category) HasSubCategory(subCategory SubCategory) bool {
	found := false
	for _, sc := range c.SubCategories {
		if sc == subCategory {
			found = true
			break
		}
	}
	return found
}

func (c Category) GetSubCategoryWithName(name string) *SubCategory {
	for _, sc := range c.SubCategories {
		if sc.String() == name {
			return &sc
		}
	}
	return nil
}

type Feature struct {
	Name          string  `json:"name"`
	Multiple      bool    `json:"multiple"`
	Fields        []Field `json:"fields"`
	EnabledFields map[Field]bool
}

type Field string

func (f Field) String() string {
	return string(f)
}

func (f *Feature) HasFiled(field Field) bool {
	found := false
	for _, ff := range f.Fields {
		if ff == field {
			found = true
			break
		}
	}
	return found
}

func (f *Feature) ToggleField(field Field) {
	if f.Multiple {
		if isEnabled, ok := f.EnabledFields[field]; ok {
			f.EnabledFields[field] = !isEnabled
		} else {
			f.EnabledFields[field] = true
		}
	} else {
		for ef := range f.EnabledFields {
			f.EnabledFields[ef] = false
		}
		f.EnabledFields[field] = true
	}
}

func (f *Feature) GetEnabledFields() []Field {
	var enabled []Field
	for ff, isEnabled := range f.EnabledFields {
		if isEnabled {
			enabled = append(enabled, ff)
		}
	}
	return enabled
}

func NewReview(rc *ReviewsCore) *Review {
	return &Review{reviewsCore: rc}
}

type Review struct {
	reviewsCore *ReviewsCore
	review      string
	name        string
	address     string
	category    Category
	subCategory SubCategory
	features    []Feature
}

func (r *Review) Category() Category {
	return r.category
}

func (r *Review) Features() []Feature {
	return r.features
}

func (r *Review) SetAddress(address string) {
	r.address = address
}

func (r *Review) SetName(name string) {
	r.name = name
}

func (r *Review) SubCategory() SubCategory {
	return r.subCategory
}

func (r *Review) SetCategory(category Category) error {
	if r.reviewsCore.HasCategory(category) {
		r.category = category
		r.subCategory = SubCategory("")
		return nil
	} else {
		return fmt.Errorf("no such category as %s in review core", category.Name)
	}
}

func (r *Review) SetSubCategory(subCategory SubCategory) error {
	if r.category.HasSubCategory(subCategory) {
		r.subCategory = subCategory
		return nil
	} else {
		return fmt.Errorf("no such sub category as %s in review core", subCategory)
	}
}

func (r *Review) HasFeature(feature Feature) bool {
	found := false
	for _, f := range r.features {
		if f.Name == feature.Name {
			found = true
			break
		}
	}
	return found
}

func (r *Review) AddFeature(feature Feature) error {
	if r.reviewsCore.HasFeature(feature) {
		if !r.HasFeature(feature) {
			r.features = append(r.features, feature)
			return nil
		} else {
			return fmt.Errorf("feature %s already added", feature.Name)
		}
	} else {
		return fmt.Errorf("no such feature as %s in review core", feature.Name)
	}
}

func (r *Review) RemoveFeature(feature Feature) {
	if r.HasFeature(feature) {
		for i, f := range r.features {
			if f.Name == feature.Name {
				r.features = append(r.features[:i], r.features[i+1:]...)
				return
			}
		}
	}
}

func (r *Review) GetInBetweenText(name string) string {
	text, err := r.reviewsCore.GetInBetweenText(name)
	if err != nil {
		logrus.Error(err)
		return ""
	}
	return text
}

func (r *Review) Replace(old, new string) {
	r.review = strings.ReplaceAll(r.review, old, new)
}

func (r *Review) GenerateReview() string {
	r.review += r.GetInBetweenText("start")
	r.review += r.GetInBetweenText("category")
	r.review += r.GetInBetweenText("sub_category")
	r.review += r.GetInBetweenText("features")
	r.review += r.GetInBetweenText("end")

	r.Replace("%name%", r.name)
	r.Replace("%category%", r.category.Name)
	r.Replace("%sub_category%", r.subCategory.String())

	var features string
	for _, f := range r.features {
		enabled := f.GetEnabledFields()
		if len(enabled) > 0 {
			if f.Multiple {
				features += f.Name + ": "
				for i, ff := range enabled {
					features += ff.String()
					if i != len(enabled)-1 {
						features += ", "
					} else {
						features += "\n"
					}
				}
			} else {
				features += f.Name + ": " + enabled[0].String() + "\n"
			}
		}
	}

	r.Replace("%features%", features)
	r.Replace("%address%", r.address)

	return r.review
}
