package post

import "time"

type Post struct {
	Id         int            `json:"id,omitempty"`
	UserId     string         `json:"user_id,omitempty"`
	Content    string         `json:"content,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	Subject    string         `json:"subject,omitempty"`
	ParentId   int            `json:"parent_id,omitempty"`
	Categories []int          `json:"categories,omitempty"`
	Comments   []PostAndMarks `json:"comments,omitempty"`
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

type CommentsAndMarks struct {
	Id        int                `json:"id,omitempty"`
	UserId    string             `json:"user_id,omitempty"`
	Content   string             `json:"content,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	ParentId  int                `json:"parent_id,omitempty"`
	UserLogin string             `json:"user_login,omitempty"`
	Likes     int                `json:"likes,omitempty"`
	Dislikes  int                `json:"dislikes,omitempty"`
	Children  []CommentsAndMarks `json:"children"`
}
