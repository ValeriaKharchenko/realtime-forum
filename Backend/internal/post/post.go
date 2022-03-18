package post

import "time"

type Post struct {
	Id         int            `json:"id"`
	UserId     string         `json:"user_id"`
	Content    string         `json:"content"`
	CreatedAt  time.Time      `json:"created_at"`
	Subject    string         `json:"subject"`
	ParentId   int            `json:"parent_id,omitempty"`
	Categories []int          `json:"categories"`
	Comments   []PostAndMarks `json:"comments"`
}

type Mark struct {
	PostId int    `json:"post_id,omitempty"`
	UserId string `json:"user_id,omitempty"`
	Mark   bool   `json:"mark,omitempty"`
}

type PostAndMarks struct {
	Post
	UserLogin  string `json:"user_login,omitempty"`
	Likes      int    `json:"likes,omitempty"`
	Dislikes   int    `json:"dislikes,omitempty"`
	Categories string `json:"categories,omitempty"`
}

type Category struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
