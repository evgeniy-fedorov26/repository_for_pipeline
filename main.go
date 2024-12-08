package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const bufferTime time.Duration = 10 * time.Second //Время опустошения буфера
const bufferSize int = 10                         //размер буфера

type RingIntBuffer struct {
	r *ring.Ring
	m sync.Mutex // Мьютекс для потокобезопасности
}

func NewRingBuffer(size int) *RingIntBuffer {
	return &RingIntBuffer{
		r: ring.New(size),
		m: sync.Mutex{},
	}
}

// Push добавляет новый элемент в буфер, затирая старые элементы при переполнении
func (r *RingIntBuffer) Push(el int) {
	r.m.Lock()
	defer r.m.Unlock()
	r.r.Value = el   // Запись значения в текущее положение
	r.r = r.r.Next() // Переход к следующей позиции
}

// Get возвращает все элементы буфера и очищает его
func (r *RingIntBuffer) Get() []int {
	r.m.Lock()
	defer r.m.Unlock()
	output := make([]int, 0, r.r.Len())
	r.r.Do(func(p interface{}) {
		if p != nil {
			output = append(output, p.(int))
		}
	})
	// Очищаем буфер
	for i := 0; i < r.r.Len(); i++ {
		r.r.Value = nil
		r.r = r.r.Next()
	}
	return output
}

type Stage func(<-chan bool, <-chan int) <-chan int

type Pipeline struct {
	stages []Stage
	done   <-chan bool
}

// инициализация нового пайплайна
func NewPipeline(done <-chan bool, stages ...Stage) *Pipeline {
	return &Pipeline{stages, done}
}

// запуск отдельной стадии
func (p *Pipeline) runStage(stage Stage, input <-chan int) <-chan int {
	return stage(p.done, input)
}

// запуск всего пайпл
func (p *Pipeline) Run(source <-chan int) <-chan int {
	var c <-chan int = source
	for index := range p.stages {
		c = p.runStage(p.stages[index], c)
	}
	return c
}

// Источник
func dataSource() (<-chan int, <-chan bool) {
	c := make(chan int)
	done := make(chan bool)
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(os.Stdin)
		var data string
		for {
			scanner.Scan()
			data = scanner.Text()
			if strings.EqualFold(data, "exit") {
				fmt.Println("Программа завершила работу!")
				close(c)
				return
			}
			i, err := strconv.Atoi(data)
			if err != nil {
				fmt.Println("Программа обрабатывает только целые числа!")
				continue
			}
			c <- i
		}
	}()
	return c, done
}

// Отсеиваем числа меньшие 0
func negativeFilterStageInt(done <-chan bool, c <-chan int) <-chan int {
	convertedIntChan := make(chan int)
	go func() {
		defer close(convertedIntChan)
		for {
			select {
			case data := <-c:
				if data > 0 {
					select {
					case convertedIntChan <- data:
					case <-done:
						return
					}
				}
			case <-done:
				return
			}
		}
	}()
	return convertedIntChan
}

// Отсеиваем числа кратные 3.
func specialFilterStageInt(done <-chan bool, c <-chan int) <-chan int {
	filteredIntChan := make(chan int)
	go func() {
		defer close(filteredIntChan)
		for {
			select {
			case data := <-c:
				if data != 0 && data%3 == 0 {
					select {
					case filteredIntChan <- data:
					case <-done:
						return
					}
				}
			case <-done:
				return
			}
		}
	}()
	return filteredIntChan
}

// Стадия буферизации
func bufferStageInt(done <-chan bool, c <-chan int) <-chan int {
	bufferedIntChan := make(chan int)
	buffer := NewRingBuffer(bufferSize)

	//добавляем элементы в массив
	go func() {
		defer close(bufferedIntChan)
		for {
			select {
			case data := <-c:
				buffer.Push(data)
			case <-done:
				return
			}
		}
	}()

	//Просмотр буфера с заданным интервалом
	// времени - bufferDrainInterval
	go func() {
		for {
			select {
			case <-time.After(bufferTime):
				bufferData := buffer.Get()

				// Если в кольцевом буфере есть данные,
				// выводим содержимое построчно
				if bufferData != nil {
					for _, data := range bufferData {
						select {
						case bufferedIntChan <- data:
						case <-done:
							return
						}
					}
				}
			case <-done:
				return
			}
		}
	}()
	return bufferedIntChan
}

// Потребитель
func consumer(done <-chan bool, c <-chan int) {
	go func() {
		for {
			select {
			case data := <-c:
				fmt.Printf("Обработаны данные: %d\n", data)
			case <-done:
				return
			}
		}
	}()
}

func main() {
	source, done := dataSource()
	pipeline := NewPipeline(done, negativeFilterStageInt, specialFilterStageInt, bufferStageInt)
	consumer(done, pipeline.Run(source))
}
