package main

import (
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "encoding/json"
    "bytes"
    "strconv"
    "mime/multipart"
    "rb-bot/line-length"
)

type WebhookResponse struct {
    Review_request struct {
        Links struct {
            Latest_diff struct {
                Href string
            }
        }
    }
}

type Link struct {
    Href   string
    Method string
}

type LinkContainer struct {
    Diffs        Link
    Latest_Diff  Link
    Patched_File Link
    Self         Link
}

type ReviewRequest struct {
    Commit_Id string
    Links     LinkContainer
}

type ReviewContainer struct {
    Stat           string
    Review_Request ReviewRequest
}

type DiffFile struct {
    Id    int
    Links LinkContainer
}

type DiffFileContainer struct {
    Files []DiffFile
}

type Chunk struct {
    RhLine int
    RhText string
    WhitespaceOnly bool
}

type DiffChunk struct {
    Index int
    Lines []Chunk
}

type Comment struct {
    Line int
    Text string
}

type FileDiff struct {
    Id        int

    Diff_Data struct {
        Chunks []DiffChunk
    }

    // Map of line to comment?
    Comments []Comment
}

type ReviewResponse struct {
    Review struct {
        Id int
    }
}

func (c *Chunk) UnmarshalJSON(bs []byte) error {
    arr := []interface{}{}
    json.Unmarshal(bs, &arr)

    fmt.Printf("Interface: %+v\n\n", arr)

    // TODO: add error handling here.
    c.RhLine         = int(arr[4].(float64))
    c.RhText         = arr[5].(string)
    c.WhitespaceOnly = arr[7].(bool)
    return nil
}

func GetLatestDiffLink() (error, string) {
    req, err := http.NewRequest("GET",
                                "http://reviews.example.com/api/" +
                                "review-requests/2/",
                                nil)

    req.Header.Add("Authorization",
                   "token 940121df848fabb83c5b02a66b6ed1da513c78ff")

    resp, err := (&http.Client{}).Do(req)

    if (err != nil) {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if (err != nil) {
        log.Fatal(err)
    }

    fmt.Printf("The body is: %s\n", body)

    var review ReviewContainer

    if (json.Valid(body)) {
        err := json.Unmarshal(body, &review)

        if err != nil {
            log.Fatal(err)
        }

        fmt.Printf("The decoded data: %+v\n\n", review)
    } else {
        fmt.Printf("Invalid json\n")
    }

    return err, review.Review_Request.Links.Latest_Diff.Href
}

func GetDiffedFiles(link string) (error, DiffFileContainer) {
    req, err := http.NewRequest("GET", link + "/files/", nil)

    req.Header.Add("Authorization",
                   "token 940121df848fabb83c5b02a66b6ed1da513c78ff")

    resp, err := (&http.Client{}).Do(req)

    if (err != nil) {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if (err != nil) {
        log.Fatal(err)
    }

    fmt.Printf("The body is: %s\n\n", body)

    var diffFiles DiffFileContainer

    if (json.Valid(body)) {
        err := json.Unmarshal(body, &diffFiles)

        if err != nil {
            log.Fatal(err)
        }
    } else {
        fmt.Printf("Invalid json\n")
    }

    return err, diffFiles
}

func GetFileDiff (link string) (error, FileDiff) {
    req, err := http.NewRequest("GET", link, nil)

    req.Header.Add("Authorization",
                   "token 940121df848fabb83c5b02a66b6ed1da513c78ff")
    req.Header.Add("Accept",
                   "application/vnd.reviewboard.org.diff.data+json")

    resp, err := (&http.Client{}).Do(req)

    if (err != nil) {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if (err != nil) {
        log.Fatal(err)
    }

    fmt.Printf("The body is: %s\n\n", body)

    var file FileDiff

    if (json.Valid(body)) {
        err := json.Unmarshal(body, &file)

        if err != nil {
            log.Fatal(err)
        }
    } else {
        fmt.Printf("Invalid json\n")
    }

    return err, file

}

func RunCheckers (files *[]FileDiff) {
    for i := 0; i< len(*files); i++ {
        for _, chunk := range (*files)[i].Diff_Data.Chunks {
            for _, diffChunk := range chunk.Lines {
                if (len(diffChunk.RhText) > 80) {
fmt.Printf("Found a too long line\n\n")
                    var comment Comment
                    comment.Line = diffChunk.RhLine
                    comment.Text = "This line is too long"
                    (*files)[i].Comments = append((*files)[i].Comments, comment)
                }
            }
        }
    }

    fmt.Printf("The comment data is %+v\n\n", files)
}

func SendComments (reviewId string, files []FileDiff) {
    var b bytes.Buffer
    w := multipart.NewWriter(&b)

    fw, err := w.CreateFormField("body_top")

    if err != nil {
        log.Fatal(err)
    }

    if _, err := fw.Write([]byte("This is a test review")); err != nil {
        log.Fatal(err)
    }

    w.Close()

    var reviewUrl string = "http://reviews.example.com/api/review-requests/" +
                           reviewId +
                           "/reviews/"

fmt.Printf("\n\nURL: %s", reviewUrl)

    // Post a new blank review, to which we will add comments
    req, err := http.NewRequest("POST", reviewUrl, &b)

    req.Header.Add("Authorization",
                   "token 940121df848fabb83c5b02a66b6ed1da513c78ff")
    req.Header.Set("Content-Type",
                   w.FormDataContentType())

    fmt.Printf("Content type: %s", w.FormDataContentType())

    resp, err := (&http.Client{}).Do(req)

    if (err != nil) {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, _ := ioutil.ReadAll(resp.Body)

    var reviewResponse ReviewResponse

    if (json.Valid(body)) {
        err := json.Unmarshal(body, &reviewResponse)

        if err != nil {
            log.Fatal(err)
        }
    } else {
        fmt.Printf("Invalid json\n")
    }

    var reviewResponseIdString string = strconv.Itoa(reviewResponse.Review.Id)

    for _, file := range files {
        if (len(file.Comments) > 0) {
            for _, comment := range file.Comments {
                var commentBuffer bytes.Buffer
                commentWriter := multipart.NewWriter(&commentBuffer)

                cfw, err := commentWriter.CreateFormField("filediff_id")

                if err != nil {
                    log.Fatal(err)
                }

                var fileId string = strconv.Itoa(file.Id)

                if _, err = cfw.Write([]byte(fileId)); err != nil {
                    log.Fatal(err)
                }

                cfw, err = commentWriter.CreateFormField("first_line")

                if err != nil {
                    log.Fatal(err)
                }

                var commentLine string = strconv.Itoa(comment.Line)

                if _, err = cfw.Write([]byte(commentLine)); err != nil {
                    log.Fatal(err)
                }

                cfw, err = commentWriter.CreateFormField("num_lines")

                if err != nil {
                    log.Fatal(err)
                }

                if _, err = cfw.Write([]byte("1")); err != nil {
                    log.Fatal(err)
                }

                cfw, err = commentWriter.CreateFormField("text")

                if err != nil {
                    log.Fatal(err)
                }

                if _, err = cfw.Write([]byte(comment.Text)); err != nil {
                    log.Fatal(err)
                }

                commentWriter.Close()

                var reviewCommentUrl string = "http://reviews.example.com/api/review-requests/" +
                                              reviewId +
                                              "/reviews/" +
                                              reviewResponseIdString +
                                              "/diff-comments/"

                fmt.Printf("Sending: %+v\n\n", commentBuffer)

                // Post the comments
                req, err = http.NewRequest("POST",
                                           reviewCommentUrl,
                                           &commentBuffer)

                req.Header.Add("Authorization",
                               "token 940121df848fabb83c5b02a66b6ed1da513c78ff")
                req.Header.Set("Content-Type",
                               commentWriter.FormDataContentType())

                resp, err = (&http.Client{}).Do(req)

                if (err != nil) {
                    log.Fatal(err)
                }
                defer resp.Body.Close()

                body, _ = ioutil.ReadAll(resp.Body)

                fmt.Printf("Response: %s\n\n", body)

            }
        }
    }


}

func Handler(w http.ResponseWriter, r *http.Request) {
}

//func main() {
//    fmt.Printf("I received a request\n\n")
//
//    var reviewId string = "2"
//
//    err, diffLink := GetLatestDiffLink()
//    if (err != nil) {
//        log.Fatal(err)
//    }
//
//    err, diff := GetDiffedFiles(diffLink)
//
//    if (err != nil) {
//        log.Fatal(err)
//    }
//
//    var diffFiles []FileDiff
//    var fileDiff    FileDiff
//
//    for _, element := range diff.Files {
//        err, fileDiff = GetFileDiff(element.Links.Self.Href)
//        fileDiff.Id = element.Id
//        diffFiles = append(diffFiles,fileDiff)
//    }
//
//    // Add comments to lines of the files
//    RunCheckers(&diffFiles)
//    SendComments(reviewId, diffFiles)
//}

func main() {
    line-length.Bar()
}

//func main() {
//    response, err := http.Get("http://golang.org/")
//    if err != nil {
//        fmt.Printf("%s", err)
//    } else {
//        defer response.Body.Close()
//        contents, err := ioutil.ReadAll(response.Body)
//        if err != nil {
//            fmt.Printf("%s", err)
//        }
//        fmt.Printf("%s\n", string(contents))
//    }
//}
