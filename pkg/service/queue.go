package service

// Queue represents a set of file batches that are to be uploaded or downloaded
type Queue struct {
	Total int      // total number of batches
	Queue []*Batch // a batch represents a collection of files to be uploaded or downloaded
}

// create a new upload/download queue
func NewQ() *Queue {
	return &Queue{
		Total: 0,
		Queue: make([]*Batch, 0),
	}
}

// NOTE: thes does NOT ensure there are no duplicate batches!
//
// that will need to be done elsewhere
func (q *Queue) Enqueue(b *Batch) {
	q.Queue = append(q.Queue, b)
	q.Total += 1
}

func (q *Queue) Dequeue() *Batch {
	if len(q.Queue) == 0 {
		return nil
	}
	item := q.Queue[0]
	q.Queue = q.Queue[1:]
	q.Total -= 1
	return item
}
