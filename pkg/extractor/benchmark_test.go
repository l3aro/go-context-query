// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkExtraction benchmarks the extraction speed for each language.
func BenchmarkGoExtractor(b *testing.B) {
	// Create sample Go files for benchmarking
	goCode := `package main

import (
	"fmt"
	"go-utils/math"
)

// helper is a local helper function
func helper() int {
	return 42
}

// main is the entry point that calls functions from other files
func main() {
	// Call local helper
	result := helper()

	// Call function from utils/math package
	sum := math.Add(1, 2)

	// Call another function from utils
	product := math.Multiply(3, 4)

	fmt.Println(result, sum, product)
}

// callerFunction calls both local and external functions
func callerFunction() int {
	a := helper()
	b := math.Add(a, 10)
	c := math.Multiply(b, 2)
	return c
}

type MyStruct struct {
	Name string
	Age  int
}

func (m MyStruct) GetName() string {
	return m.Name
}

type Reader interface {
	Read(p []byte) (n int, err error)
}
`
	tmpDir := b.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, []byte(goCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewGoExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(goFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkTypeScriptExtractor(b *testing.B) {
	// Create sample TypeScript files for benchmarking
	tsCode := `import { add, multiply } from './utils/math';

export interface User {
    name: string;
    age: number;
}

export class Helper {
    private value: number = 0;
    
    public getValue(): number {
        return this.value;
    }
    
    public setValue(v: number): void {
        this.value = v;
    }
    
    public process(items: number[]): number[] {
        return items.map(item => item * 2);
    }
}

export function helper(): number {
    return 42;
}

export function main(): void {
    const h = new Helper();
    const data = [1, 2, 3, 4, 5];
    const result = h.process(data);
    console.log(result);
}

export interface Processor {
    process(): void;
    calculate(a: number, b: number): number;
}
`
	tmpDir := b.TempDir()
	tsFile := filepath.Join(tmpDir, "main.ts")
	if err := os.WriteFile(tsFile, []byte(tsCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewTypeScriptExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(tsFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkPythonExtractor(b *testing.B) {
	// Create a sample Python file for benchmarking
	pythonCode := `package main

import os
import sys
from typing import List, Dict, Optional

class MyClass:
    def __init__(self):
        self.value = 0
    
    def process(self, data: List[int]) -> Dict[str, int]:
        result = {}
        for item in data:
            result[f"key_{item}"] = item * 2
        return result

def process_items(items: List[str]) -> List[str]:
    return [item.upper() for item in items]

def calculate(a: int, b: int) -> Optional[int]:
    if b == 0:
        return None
    return a / b

class AnotherClass:
    def method1(self, x: int) -> int:
        return x * 2
    
    def method2(self, y: str) -> str:
        return y.strip()

def main():
    obj = MyClass()
    data = [1, 2, 3, 4, 5]
    result = obj.process(data)
    print(result)

if __name__ == "__main__":
    main()
`
	tmpDir := b.TempDir()
	pythonFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(pythonFile, []byte(pythonCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewPythonExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(pythonFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkJavaExtractor(b *testing.B) {
	// Create a sample Java file for benchmarking
	javaCode := `package com.example;

import java.util.*;
import java.io.*;

public class MyClass {
    private int value;
    private String name;
    
    public MyClass() {
        this.value = 0;
        this.name = "";
    }
    
    public MyClass(int value, String name) {
        this.value = value;
        this.name = name;
    }
    
    public Map<String, Integer> process(List<Integer> data) {
        Map<String, Integer> result = new HashMap<>();
        for (Integer item : data) {
            result.put("key_" + item, item * 2);
        }
        return result;
    }
    
    public int getValue() {
        return value;
    }
    
    public void setValue(int value) {
        this.value = value;
    }
}

interface Processor {
    void process();
    int calculate(int a, int b);
}

class AnotherClass implements Processor {
    @Override
    public void process() {
        System.out.println("Processing");
    }
    
    @Override
    public int calculate(int a, int b) {
        return a + b;
    }
}
`
	tmpDir := b.TempDir()
	javaFile := filepath.Join(tmpDir, "Test.java")
	if err := os.WriteFile(javaFile, []byte(javaCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewJavaExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(javaFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkRustExtractor(b *testing.B) {
	// Create a sample Rust file for benchmarking
	rustCode := `use std::collections::HashMap;

struct MyStruct {
    value: i32,
    name: String,
}

impl MyStruct {
    fn new() -> Self {
        MyStruct {
            value: 0,
            name: String::new(),
        }
    }
    
    fn process(&self, data: Vec<i32>) -> HashMap<String, i32> {
        let mut result = HashMap::new();
        for item in data {
            result.insert(format!("key_{}", item), item * 2);
        }
        result
    }
    
    fn get_value(&self) -> i32 {
        self.value
    }
}

trait Processor {
    fn process(&self);
    fn calculate(&self, a: i32, b: i32) -> i32;
}

struct AnotherStruct;

impl Processor for AnotherStruct {
    fn process(&self) {
        println!("Processing");
    }
    
    fn calculate(&self, a: i32, b: i32) -> i32 {
        a + b
    }
}

fn main() {
    let s = MyStruct::new();
    let data = vec![1, 2, 3, 4, 5];
    let result = s.process(data);
    println!("{:?}", result);
}
`
	tmpDir := b.TempDir()
	rustFile := filepath.Join(tmpDir, "test.rs")
	if err := os.WriteFile(rustFile, []byte(rustCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewRustExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(rustFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkJavaScriptExtractor(b *testing.B) {
	// Create a sample JavaScript file for benchmarking
	jsCode := "class MyClass {\n" +
		"    constructor() {\n" +
		"        this.value = 0;\n" +
		"        this.name = '';\n" +
		"    }\n" +
		"\n" +
		"    process(data) {\n" +
		"        const result = {};\n" +
		"        for (const item of data) {\n" +
		"            result['key_' + item] = item * 2;\n" +
		"        }\n" +
		"        return result;\n" +
		"    }\n" +
		"\n" +
		"    getValue() {\n" +
		"        return this.value;\n" +
		"    }\n" +
		"\n" +
		"    setValue(value) {\n" +
		"        this.value = value;\n" +
		"    }\n" +
		"}\n" +
		"\n" +
		"function processItems(items) {\n" +
		"    return items.map(item => item.toUpperCase());\n" +
		"}\n" +
		"\n" +
		"function calculate(a, b) {\n" +
		"    if (b === 0) {\n" +
		"        return null;\n" +
		"    }\n" +
		"    return a / b;\n" +
		"}\n" +
		"\n" +
		"const obj = new MyClass();\n" +
		"const data = [1, 2, 3, 4, 5];\n" +
		"const result = obj.process(data);\n" +
		"console.log(result);\n" +
		"\n" +
		"module.exports = { MyClass, processItems, calculate };\n"
	tmpDir := b.TempDir()
	jsFile := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(jsFile, []byte(jsCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewJavaScriptExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(jsFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkCExtractor(b *testing.B) {
	// Create a sample C file for benchmarking
	cCode := `#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    int value;
    char name[100];
} MyStruct;

typedef struct Node {
    int data;
    struct Node* next;
} Node;

void process_array(int* arr, int size) {
    for (int i = 0; i < size; i++) {
        arr[i] = arr[i] * 2;
    }
}

int calculate(int a, int b) {
    if (b == 0) {
        return -1;
    }
    return a / b;
}

MyStruct* create_struct(int value, const char* name) {
    MyStruct* s = (MyStruct*)malloc(sizeof(MyStruct));
    s->value = value;
    strcpy(s->name, name);
    return s;
}

void free_struct(MyStruct* s) {
    free(s);
}

int main() {
    int arr[] = {1, 2, 3, 4, 5};
    process_array(arr, 5);
    MyStruct* s = create_struct(10, "test");
    printf("Value: %d\\n", s->value);
    free_struct(s);
    return 0;
}
`
	tmpDir := b.TempDir()
	cFile := filepath.Join(tmpDir, "test.c")
	if err := os.WriteFile(cFile, []byte(cCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewCExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(cFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkCPPExtractor(b *testing.B) {
	// Create a sample C++ file for benchmarking
	cppCode := `#include <iostream>
#include <vector>
#include <map>
#include <string>
#include <memory>

class MyClass {
private:
    int value;
    std::string name;
    
public:
    MyClass() : value(0), name("") {}
    MyClass(int v, const std::string& n) : value(v), name(n) {}
    
    std::map<std::string, int> process(const std::vector<int>& data) {
        std::map<std::string, int> result;
        for (int item : data) {
            result["key_" + std::to_string(item)] = item * 2;
        }
        return result;
    }
    
    int getValue() const { return value; }
    void setValue(int v) { value = v; }
};

class AnotherClass {
public:
    virtual void process() = 0;
    virtual int calculate(int a, int b) = 0;
};

class ConcreteClass : public AnotherClass {
public:
    void process() override {
        std::cout << "Processing" << std::endl;
    }
    
    int calculate(int a, int b) override {
        return a + b;
    }
};

int main() {
    MyClass obj;
    std::vector<int> data = {1, 2, 3, 4, 5};
    auto result = obj.process(data);
    std::cout << "Done" << std::endl;
    return 0;
}
`
	tmpDir := b.TempDir()
	cppFile := filepath.Join(tmpDir, "test.cpp")
	if err := os.WriteFile(cppFile, []byte(cppCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewCPPExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(cppFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkRubyExtractor(b *testing.B) {
	// Create a sample Ruby file for benchmarking
	rubyCode := `class MyClass
  def initialize
    @value = 0
    @name = ''
  end
  
  def process(data)
    result = {}
    data.each do |item|
      result["key_#{item}"] = item * 2
    end
    result
  end
  
  def value
    @value
  end
  
  def value=(v)
    @value = v
  end
end

module Processor
  def process
    puts "Processing"
  end
  
  def calculate(a, b)
    a + b
  end
end

obj = MyClass.new
data = [1, 2, 3, 4, 5]
result = obj.process(data)
puts result.inspect
`
	tmpDir := b.TempDir()
	rubyFile := filepath.Join(tmpDir, "test.rb")
	if err := os.WriteFile(rubyFile, []byte(rubyCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewRubyExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(rubyFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkPHPExtractor(b *testing.B) {
	// Create a sample PHP file for benchmarking
	phpCode := `<?php

class MyClass {
    private $value = 0;
    private $name = '';
    
    public function __construct() {
    }
    
    public function process(array $data): array {
        $result = [];
        foreach ($data as $item) {
            $result["key_{$item}"] = $item * 2;
        }
        return $result;
    }
    
    public function getValue(): int {
        return $this->value;
    }
    
    public function setValue(int $value): void {
        $this->value = $value;
    }
}

interface Processor {
    public function process(): void;
    public function calculate(int $a, int $b): int;
}

class AnotherClass implements Processor {
    public function process(): void {
        echo "Processing\n";
    }
    
    public function calculate(int $a, int $b): int {
        return $a + $b;
    }
}

$obj = new MyClass();
$data = [1, 2, 3, 4, 5];
$result = $obj->process($data);
var_dump($result);
`
	tmpDir := b.TempDir()
	phpFile := filepath.Join(tmpDir, "test.php")
	if err := os.WriteFile(phpFile, []byte(phpCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewPHPExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(phpFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkKotlinExtractor(b *testing.B) {
	// Create a sample Kotlin file for benchmarking
	kotlinCode := `package com.example

import java.util.*

class MyClass {
    var value: Int = 0
    var name: String = ""
    
    constructor() {
        this.value = 0
        this.name = ""
    }
    
    constructor(value: Int, name: String) {
        this.value = value
        this.name = name
    }
    
    fun process(data: List<Int>): Map<String, Int> {
        val result = HashMap<String, Int>()
        for (item in data) {
            result["key_$item"] = item * 2
        }
        return result
    }
    
    fun getValue(): Int = value
    
    fun setValue(value: Int) {
        this.value = value
    }
}

interface Processor {
    fun process()
    fun calculate(a: Int, b: Int): Int
}

class AnotherClass : Processor {
    override fun process() {
        println("Processing")
    }
    
    override fun calculate(a: Int, b: Int): Int {
        return a + b
    }
}

fun main() {
    val obj = MyClass()
    val data = listOf(1, 2, 3, 4, 5)
    val result = obj.process(data)
    println(result)
}
`
	tmpDir := b.TempDir()
	kotlinFile := filepath.Join(tmpDir, "Test.kt")
	if err := os.WriteFile(kotlinFile, []byte(kotlinCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewKotlinExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(kotlinFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}

func BenchmarkCSharpExtractor(b *testing.B) {
	// Create a sample C# file for benchmarking
	csharpCode := "using System;\n" +
		"using System.Collections.Generic;\n" +
		"using System.Linq;\n" +
		"\n" +
		"namespace Example\n" +
		"{\n" +
		"    public class MyClass\n" +
		"    {\n" +
		"        public int Value { get; set; }\n" +
		"        public string Name { get; set; }\n" +
		"        \n" +
		"        public MyClass()\n" +
		"        {\n" +
		"            this.Value = 0;\n" +
		"            this.Name = \"\";\n" +
		"        }\n" +
		"        \n" +
		"        public MyClass(int value, string name)\n" +
		"        {\n" +
		"            this.Value = value;\n" +
		"            this.Name = name;\n" +
		"        }\n" +
		"        \n" +
		"        public Dictionary<string, int> Process(List<int> data)\n" +
		"        {\n" +
		"            var result = new Dictionary<string, int>();\n" +
		"            foreach (var item in data)\n" +
		"            {\n" +
		"                result[\"key_\" + item] = item * 2;\n" +
		"            }\n" +
		"            return result;\n" +
		"        }\n" +
		"    }\n" +
		"    \n" +
		"    public interface IProcessor\n" +
		"    {\n" +
		"        void Process();\n" +
		"        int Calculate(int a, int b);\n" +
		"    }\n" +
		"    \n" +
		"    public class AnotherClass : IProcessor\n" +
		"    {\n" +
		"        public void Process()\n" +
		"        {\n" +
		"            Console.WriteLine(\"Processing\");\n" +
		"        }\n" +
		"        \n" +
		"        public int Calculate(int a, int b)\n" +
		"        {\n" +
		"            return a + b;\n" +
		"        }\n" +
		"    }\n" +
		"    \n" +
		"    class Program\n" +
		"    {\n" +
		"        static void Main(string[] args)\n" +
		"        {\n" +
		"            var obj = new MyClass();\n" +
		"            var data = new List<int> { 1, 2, 3, 4, 5 };\n" +
		"            var result = obj.Process(data);\n" +
		"            Console.WriteLine(string.Join(\", \", result));\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	tmpDir := b.TempDir()
	csharpFile := filepath.Join(tmpDir, "Test.cs")
	if err := os.WriteFile(csharpFile, []byte(csharpCode), 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.Run("sample_file", func(b *testing.B) {
		extractor := NewCSharpExtractor()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := extractor.Extract(csharpFile)
			if err != nil {
				b.Fatalf("Extract failed: %v", err)
			}
		}
	})
}
