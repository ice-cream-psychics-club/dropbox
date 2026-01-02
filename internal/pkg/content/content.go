package content

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type Submission struct {
	Time            time.Time
	Member          Member
	Title           string
	Creators        string
	ReleaseYear     string
	Format          string
	Genre           string
	Length          string
	Approachability int
	Topics          []string
	Hook            string
}

func (s *Submission) Record() []string {
	return []string{
		s.Time.Format(time.RFC3339),
		string(s.Member),
		s.Title,
		s.Creators,
		s.ReleaseYear,
		s.Format,
		s.Genre,
		s.Length,
		fmt.Sprintf("%d", s.Approachability),
		strings.Join(s.Topics, ", "),
		s.Hook,
	}
}

type SubmissionID struct {
	Title     string
	Submitter Member
}

type Rating struct {
	Rater    Member
	Interest int
	SubmissionID
}

type Member string

var submissionsHeader = []string{
	"time",
	"member",
	"title",
	"creators",
	"release/publication year",
	"format",
	"genre",
	"length",
	"approachability",
	"topics",
	"hook",
}

func ImportSubmissions(r io.Reader) ([]Submission, error) {
	csvReader := csv.NewReader(r)

	submissions := make([]Submission, 0)
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading record %d: %w", len(submissions), err)
		}

		if len(record) != 11 {
			return nil, fmt.Errorf("record %d has too few columns, want at least %d, got %d", len(submissions), 11, len(record))
		}

		submitTime, err := time.Parse(time.RFC3339, record[0])
		if err != nil {
			return nil, fmt.Errorf("invalid submission time at row %d: %w", len(submissions), err)
		}

		approachability, err := strconv.Atoi(record[8])
		if err != nil {
			return nil, fmt.Errorf("invalid approachability rating at row %d: %w", len(submissions), err)
		}

		topics := strings.Split(record[9], ",")
		for i := range topics {
			topics[i] = strings.TrimSpace(topics[i])
		}

		submissions = append(submissions, Submission{
			Time:            submitTime,
			Member:          Member(record[1]),
			Title:           record[2],
			Creators:        record[3],
			ReleaseYear:     record[4],
			Format:          record[5],
			Genre:           record[6],
			Length:          record[7],
			Approachability: approachability,
			Topics:          topics,
			Hook:            record[10],
		})
	}

	return submissions, nil
}

func ExportSubmissions(header []string, submissions []Submission, w io.Writer) error {
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(submissionsHeader); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	for i, s := range submissions {
		if err := csvWriter.Write(s.Record()); err != nil {
			return fmt.Errorf("error writing record %d: %w", i, err)
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

func ImportRatings(r io.Reader) (map[SubmissionID][]Rating, []Member, error) {
	csvReader := csv.NewReader(r)
	header, err := csvReader.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, io.ErrUnexpectedEOF
	}
	if err != nil {
		return nil, nil, fmt.Errorf("error reading header: %w", err)
	}

	members := make([]Member, len(header)-2)
	for i := 2; i < len(header); i++ {
		members[i-2] = Member(header[i])
	}

	ratings := make(map[SubmissionID][]Rating, len(header)-2)
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error reading record %d: %w", len(ratings)+1, err)
		}

		submissionID := SubmissionID{
			Title:     record[0],
			Submitter: Member(record[1]),
		}

		for i := 2; i < len(record); i++ {
			member := members[i-2]

			interest := -1
			if record[i] != "TODO" {
				interest, err = strconv.Atoi(record[i])
				if err != nil {
					return nil, nil, fmt.Errorf("error parsing interest in record %d: %w", len(ratings)+1, err)
				}
			}

			ratings[submissionID] = append(ratings[submissionID], Rating{
				Rater:        member,
				Interest:     interest,
				SubmissionID: submissionID,
			})
		}
	}

	return ratings, members, nil
}

func ExportRatings(ratings map[SubmissionID][]Rating, members []Member, w io.Writer) error {
	csvWriter := csv.NewWriter(w)

	header := []string{"title", "submitter"}
	for _, m := range members {
		header = append(header, string(m))
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	count := 1
	for id, r := range ratings {
		record := make([]string, 2+len(members))
		record[0] = id.Title
		record[1] = string(id.Submitter)

		for i := range r {
			interest := "TODO"
			if r[i].Interest != -1 {
				interest = fmt.Sprintf("%d", r[i].Interest)
			}
			record = append(record, interest)
		}

		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("error writing record %d: %w", count, err)
		}

		count++
	}

	csvWriter.Flush()
	return csvWriter.Error()

}

func CalculateDelta(before, after []Submission) []Submission {
	beforeIDs := make(map[SubmissionID]Submission)
	afterIDs := make(map[SubmissionID]Submission)

	for _, v := range before {
		beforeIDs[SubmissionID{
			Title:     v.Title,
			Submitter: v.Member,
		}] = v
	}

	for _, v := range after {
		afterIDs[SubmissionID{
			Title:     v.Title,
			Submitter: v.Member,
		}] = v
	}

	delta := make([]Submission, 0, len(afterIDs))
	for id, s := range afterIDs {
		if _, ok := beforeIDs[id]; !ok {
			delta = append(delta, s)
		}
	}

	return delta
}

// func AddSubmissionsToRate(ratings map[SubmissionID][]Rating, submissions []Submission, members []Member) {
// 	ratedIDs := make(map[SubmissionID]struct{})
// 	for _, r := range ratings {
// 		ratedIDs[r.SubmissionID] = struct{}{}
// 	}

// 	for _, s := range submissions {
// 		id := SubmissionID{Title: s.Title, Submitter: s.Member}
// 		if _, ok := ratedIDs[id]; ok {
// 			continue
// 		}

// 		for _, m := range members {
// 			ratings = append(ratings, Rating{
// 				Rater:        m,
// 				Interest:     -1,
// 				SubmissionID: id,
// 			})
// 		}
// 	}
// }
