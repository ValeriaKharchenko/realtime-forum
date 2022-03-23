package post

import (
	"database/sql"
	"errors"
	"fmt"
	"forum/internal/common"
	"log"
	"strings"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) NewPost(post Post) (Post, error) {
	trimmedPost := strings.TrimSpace(post.Content)
	if trimmedPost == "" {
		return Post{}, common.InvalidArgumentError(nil, "you are trying to create an empty post")
	}
	post.Content = trimmedPost
	if post.Subject == "" && post.ParentId == 0 {
		return Post{}, common.InvalidArgumentError(nil, "topic is missing")
	}
	if len(post.Categories) == 0 && post.ParentId == 0 {
		return Post{}, common.InvalidArgumentError(nil, "category is missing")
	}
	posts, err := s.addToDB(post)
	if err != nil {
		return Post{}, err
	}
	return posts, nil
}

var (
	postCol   = "user_id, content, subject, parent_id"
	markerCol = "post_id, user_id, mark"
)

func (s *Service) addToDB(p Post) (Post, error) {
	var id *int
	if p.ParentId != 0 {
		id = &p.ParentId
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Post{}, common.DataBaseError(err)
	}
	query := fmt.Sprintf(`INSERT INTO posts (%s) VALUES ($1, $2, $3, $4) returning id, created_at`, postCol)
	row := tx.QueryRow(query, p.UserId, p.Content, p.Subject, id)
	err = row.Scan(&p.Id, &p.CreatedAt)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			common.ErrorLogger.Println(err)
			return Post{}, common.DataBaseError(err)
		}
		common.ErrorLogger.Println(err)
		return Post{}, common.SystemError(err)
	}
	if len(p.Categories) != 0 {
		var res []string
		for _, v := range p.Categories {
			r := fmt.Sprintf("(%v, %v)", p.Id, v)
			res = append(res, r)
		}
		query = fmt.Sprintf(`INSERT INTO posts_categories (post_id, category_id) VALUES %s`, strings.Join(res, ", "))
		if _, err := tx.Exec(query); err != nil {
			if err := tx.Rollback(); err != nil {
				common.ErrorLogger.Println(err)
				return Post{}, common.DataBaseError(err)
			}
			common.ErrorLogger.Println(err)
			return Post{}, common.SystemError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		common.ErrorLogger.Println(err)
		return Post{}, common.DataBaseError(err)
	}
	return p, nil
}

func (s *Service) ShowAll() ([]PostAndMarks, error) {

	query := fmt.Sprintf(`SELECT p.id,
       p.user_id,
       u.login,
       p.content,
       p.subject,
       p.created_at,
       COALESCE(p.parent_id, 0)      as parent_id,
       coalesce(dislike, 0)          as dislike,
       coalesce(like, 0)             as dislike,
       group_concat(distinct c.name) as category_name
FROM posts p
         LEFT JOIN (
    Select post_id,
           sum(case when not mark then 1 else 0 end) AS dislike,
           sum(case when mark then 1 else 0 end)     AS like
    FROM likes_dislikes
    group by post_id
) as ld ON p.id = ld.post_id
         INNER JOIN posts_categories pc on p.id = pc.post_id
         INNER JOIN categories c on c.id = pc.category_id
         INNER JOIN users u on u.id = p.user_id
WHERE p.parent_id is null
group by p.id`)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Read into
	var posts []PostAndMarks
	for rows.Next() {
		p := PostAndMarks{}
		err := rows.Scan(&p.Id, &p.Post.UserId, &p.UserLogin, &p.Content, &p.Subject, &p.CreatedAt, &p.ParentId, &p.Dislikes, &p.Likes, &p.Categories)
		if err != nil {
			// if database cannot read row
			common.InfoLogger.Println(err)
			continue
		}
		posts = append(posts, p)
	}
	return posts, nil
}

func (s *Service) AddMark(m Mark) (*Mark, error) {
	mk, err := s.getMark(m)
	if err != nil {
		return nil, common.SystemError(err)
	}

	switch {
	case mk == nil:
		err = s.addMark(m)
	case *mk == m.Mark:
		err = s.deleteMark(m)
		if err == nil {
			return nil, nil
		}
	case *mk != m.Mark:
		err = s.updateMark(m)
	}

	if err != nil {
		return nil, common.SystemError(err)
	}
	return &m, nil
}

func (s *Service) getMark(m Mark) (*bool, error) {
	query := fmt.Sprintf("SELECT mark FROM likes_dislikes WHERE post_id=$1 and user_id=$2")
	row := s.db.QueryRow(query, m.PostId, m.UserId)
	var mark *bool
	if err := row.Scan(&mark); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return mark, nil
}

func (s *Service) addMark(m Mark) error {
	query := fmt.Sprintf("INSERT INTO likes_dislikes (%s) VALUES ($1, $2, $3)", markerCol)
	if _, err := s.db.Exec(query, m.PostId, m.UserId, m.Mark); err != nil {
		return err
	}
	return nil
}

func (s *Service) updateMark(m Mark) error {
	query := fmt.Sprintf("UPDATE likes_dislikes SET mark=$1 WHERE post_id=$2 and user_id=$3")
	if _, err := s.db.Exec(query, m.Mark, m.PostId, m.UserId); err != nil {
		return err
	}
	return nil
}

func (s *Service) deleteMark(m Mark) error {
	query := fmt.Sprintf("DELETE FROM likes_dislikes WHERE post_id=$1 and user_id=$2")
	if _, err := s.db.Exec(query, m.PostId, m.UserId); err != nil {
		return err
	}
	return nil
}

func (s *Service) FindByCategory(catID int) ([]PostAndMarks, error) {
	query := `SELECT p.id,
       p.user_id,
       u.login,
       p.content,
       p.subject,
       p.created_at,
       COALESCE(p.parent_id, 0)      as parent_id,
       coalesce(dislike, 0)          as dislike,
       coalesce(like, 0)             as dislike,
       group_concat(distinct c.name) as category_name
FROM posts p
         LEFT JOIN (
    Select post_id,
           sum(case when not mark then 1 else 0 end) AS dislike,
           sum(case when mark then 1 else 0 end)     AS like
    FROM likes_dislikes
    group by post_id
) as ld ON p.id = ld.post_id
         INNER JOIN posts_categories pc on p.id = pc.post_id
         INNER JOIN categories c on c.id = pc.category_id
         INNER JOIN users u on u.id = p.user_id
		WHERE p.parent_id is null and p.id IN (SELECT post_id FROM posts_categories WHERE category_id=$1)
		group by p.id`
	rows, err := s.db.Query(query, catID)
	if err != nil {
		return nil, common.SystemError(err)
	}
	defer rows.Close()
	var posts []PostAndMarks
	for rows.Next() {
		var post PostAndMarks
		err := rows.Scan(&post.Id, &post.UserId, &post.UserLogin, &post.Content, &post.Subject, &post.CreatedAt, &post.ParentId, &post.Dislikes, &post.Likes, &post.Categories)
		if err != nil {
			common.ErrorLogger.Println(err)
			continue
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (s *Service) FindByUser(userID string) ([]PostAndMarks, error) {
	query := fmt.Sprintf(`SELECT p.id,
       p.user_id,
       u.login,
       p.content,
       p.subject,
       p.created_at,
       COALESCE(p.parent_id, 0)      as parent_id,
       coalesce(dislike, 0)          as dislike,
       coalesce(like, 0)             as dislike,
       group_concat(distinct c.name) as category_name
FROM posts p
         LEFT JOIN (
    Select post_id,
           sum(case when not mark then 1 else 0 end) AS dislike,
           sum(case when mark then 1 else 0 end)     AS like
    FROM likes_dislikes
    group by post_id
) as ld ON p.id = ld.post_id
         INNER JOIN posts_categories pc on p.id = pc.post_id
         INNER JOIN categories c on c.id = pc.category_id
         INNER JOIN users u on u.id = p.user_id
WHERE u.id =$1
group by p.id
ORDER BY p.created_at desc`)
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, common.SystemError(err)
	}
	defer rows.Close()
	var posts []PostAndMarks
	for rows.Next() {
		var post PostAndMarks
		err := rows.Scan(&post.Id, &post.UserId, &post.UserLogin, &post.Content, &post.Subject, &post.CreatedAt, &post.ParentId, &post.Dislikes, &post.Likes, &post.Categories)
		if err != nil {
			common.ErrorLogger.Println(err)
			continue
		}
		posts = append(posts, post)
	}

	if len(posts) == 0 {
		return nil, common.InvalidArgumentError(nil, "user has no posts")
	}
	return posts, nil
}

func (s *Service) FindAllLiked(userID string) ([]PostAndMarks, error) {
	query := `SELECT p.id,
       p.user_id,
       u.login,
       p.content,
       p.subject,
       p.created_at,
       COALESCE(p.parent_id, 0)      as parent_id,
       coalesce(dislike, 0)          as dislike,
       coalesce(like, 0)             as dislike,
       group_concat(distinct c.name) as category_name
FROM posts p
         LEFT JOIN (
    Select post_id,
           sum(case when not mark then 1 else 0 end) AS dislike,
           sum(case when mark then 1 else 0 end)     AS like
    FROM likes_dislikes
    group by post_id
) as ld ON p.id = ld.post_id
         INNER JOIN posts_categories pc on p.id = pc.post_id
         INNER JOIN categories c on c.id = pc.category_id
         INNER JOIN users u on u.id = p.user_id
WHERE p.parent_id is null and p.id IN (SELECT post_id FROM likes_dislikes WHERE mark=1 and user_id=$1)
group by p.id
ORDER BY p.created_at desc`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, common.SystemError(err)
	}
	defer rows.Close()
	var posts []PostAndMarks
	for rows.Next() {
		var post PostAndMarks
		err := rows.Scan(&post.Id, &post.UserId, &post.UserLogin, &post.Content, &post.Subject, &post.CreatedAt, &post.ParentId, &post.Dislikes, &post.Likes, &post.Categories)
		if err != nil {
			common.ErrorLogger.Println(err)
			continue
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (s *Service) ShowAllCategories() ([]Category, error) {
	query := fmt.Sprintf("SELECT id, name FROM categories")
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, common.DataBaseError(err)
	}
	var categories []Category

	for rows.Next() {
		var s Category
		if err := rows.Scan(&s.Id, &s.Name); err != nil {
			log.Println(err)
			continue
		}
		categories = append(categories, s)
	}
	if len(categories) == 0 {
		return nil, common.NotFoundError(nil, "no categories were found")
	}
	return categories, nil
}

func (s *Service) FindById(postID int) (PostAndMarks, error) {
	query := `SELECT p.id,
       p.user_id,
       u.login,
       p.content,
       p.subject,
       p.created_at,
       COALESCE(p.parent_id, 0)      as parent_id,
       coalesce(dislike, 0)          as dislike,
       coalesce(like, 0)             as dislike,
       group_concat(distinct c.name) as category_name
FROM posts p
         LEFT JOIN (
    Select post_id,
           sum(case when not mark then 1 else 0 end) AS dislike,
           sum(case when mark then 1 else 0 end)     AS like
    FROM likes_dislikes
    group by post_id
) as ld ON p.id = ld.post_id
         INNER JOIN posts_categories pc on p.id = pc.post_id
         INNER JOIN categories c on c.id = pc.category_id
         INNER JOIN users u on u.id = p.user_id
WHERE p.id =$1`
	row := s.db.QueryRow(query, postID)
	var post PostAndMarks
	err := row.Scan(&post.Id, &post.UserId, &post.UserLogin, &post.Content, &post.Subject, &post.CreatedAt, &post.ParentId, &post.Dislikes, &post.Likes, &post.Categories)
	if err != nil {
		return PostAndMarks{}, common.NotFoundError(err, "cannot find post")
	}
	//post.Comments, err = s.CommentsByPostId(post)
	//if err != nil {
	//	common.ErrorLogger.Println(err)
	//}
	return post, nil
}

func (s *Service) CommentsByPostId(postId int) ([]PostAndMarks, error) {
	comments, err := s.findComments(postId)
	if err != nil {
		return nil, err
	}
	if len(comments) == 0 {
		return nil, nil
	}

	parent, err := s.FindById(postId)
	if err != nil {
		return nil, err
	}

	//comments = append(comments, parent)
	m := make(map[int][]PostAndMarks)
	for _, p := range comments {
		m[p.ParentId] = append(m[p.ParentId], p)
	}
	addNestedChild(m, &parent)
	return parent.Comments, nil
}

func (s *Service) findComments(id int) ([]PostAndMarks, error) {
	query := fmt.Sprintf(`with recursive cte (id, user_id, parent_id, content, created_at) as (
    select id, user_id, parent_id, content, created_at
    from posts
    where parent_id =$1
    union all
    select p.id,
           p.user_id,
           p.parent_id,
           p.content,
           p.created_at
    from posts p
             inner join cte on p.parent_id = cte.id
)
select cte.id,
       cte.user_id,
       u.login,
       cte.content,
       cte.created_at,
       cte.parent_id,
       coalesce(sum(case when not ld.mark then 1 else 0 end), 0) AS dislike,
       coalesce(sum(case when ld.mark then 1 else 0 end), 0)     AS like
from cte
         LEFT JOIN users u on cte.user_id = u.id
         LEFT JOIN likes_dislikes ld on cte.id = ld.post_id
group by cte.id`)
	rows, err := s.db.Query(query, id)
	if err != nil {
		return nil, common.SystemError(err)
	}
	defer rows.Close()

	var comments []PostAndMarks
	for rows.Next() {
		var p PostAndMarks
		if err := rows.Scan(&p.Id, &p.UserId, &p.UserLogin, &p.Content, &p.CreatedAt, &p.ParentId, &p.Dislikes, &p.Likes); err != nil {
			common.WarningLogger.Println(err)
			continue
		}
		comments = append(comments, p)
	}
	return comments, nil
}

func addNestedChild(m map[int][]PostAndMarks, post *PostAndMarks) {
	children := m[post.Id]
	if len(children) == 0 {
		return
	}
	post.Comments = children
	for i := range post.Comments {
		addNestedChild(m, &post.Comments[i])
	}
}
