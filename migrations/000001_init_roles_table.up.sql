create table if not exists roles (
    id serial primary key,
    title varchar(255) not null,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

insert into roles(title) values('Администратор'),
                               ('Ментор');