package quiz

// Split divides a quiz with more than threshold species into the fewest parts of at most
// maxSize species, balanced in size and keeping species order. Every part keeps the quiz
// header. A quiz at or under threshold is returned unchanged as the only part.
func Split(q Quiz, threshold, maxSize int) []Quiz {
	total := len(q.Species)
	if total <= threshold || maxSize < 1 {
		return []Quiz{q}
	}
	parts := (total + maxSize - 1) / maxSize
	base, extra := total/parts, total%parts

	result := make([]Quiz, 0, parts)
	start := 0
	for i := 0; i < parts; i++ {
		size := base
		if i < extra {
			size++
		}
		part := q
		part.Species = append([]Species(nil), q.Species[start:start+size]...)
		result = append(result, part)
		start += size
	}
	return result
}
