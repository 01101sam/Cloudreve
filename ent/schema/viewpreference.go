package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ViewPreference holds the schema definition for the ViewPreference entity.
type ViewPreference struct {
	ent.Schema
}

// Mixin of the ViewPreference.
func (ViewPreference) Mixin() []ent.Mixin {
	return []ent.Mixin{
		CommonMixin{},
	}
}

// Fields of the ViewPreference.
func (ViewPreference) Fields() []ent.Field {
	return []ent.Field{
		field.String("folder_path").
			NotEmpty().
			Comment("The folder path this preference applies to"),
		field.String("layout").
			Default("grid").
			Comment("View layout (grid/list/gallery)"),
		field.Bool("show_thumb").
			Default(true).
			Comment("Show thumbnails in grid view"),
		field.String("sort_by").
			Default("created_at").
			Comment("Sort field"),
		field.String("sort_direction").
			Default("asc").
			Comment("Sort direction (asc/desc)"),
		field.Int("page_size").
			Default(100).
			Comment("Pagination size"),
		field.Int("gallery_width").
			Default(220).
			Comment("Gallery view image width"),
		field.String("list_columns").
			Default("").
			Comment("List view column settings as JSON string"),
	}
}

// Edges of the ViewPreference.
func (ViewPreference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("view_preferences").
			Required().
			Unique(),
	}
}

// Indexes of the ViewPreference.
func (ViewPreference) Indexes() []ent.Index {
	return []ent.Index{
		// Composite unique index on user and folder_path
		index.Fields("folder_path").
			Edges("user").
			Unique(),
	}
}
