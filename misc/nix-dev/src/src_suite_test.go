package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSrc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Src Suite")
}

var _ = Describe("nix-dev", func() {
	BeforeEach(func() {
	})
	Describe("As a developer", func() {
		Context("When I create a new project", func() {
			It("should be a novel", func() {
				Expect("foo").To(Equal("bar"))
			})
		})

		/*
		   Context("With fewer than 300 pages", func() {
		       It("should be a short story", func() {
		           Expect(shortBook.CategoryByLength()).To(Equal("SHORT STORY"))
		       })
		   })
		*/
	})

})
