--
-- PostgreSQL database dump
--


-- Dumped from database version 17.11 (Ubuntu 17.11-1.pgdg24.04+2)
-- Dumped by pg_dump version 17.11 (Ubuntu 17.11-1.pgdg24.04+2)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: auction_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.auction_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    auction_id uuid NOT NULL,
    sequence_number bigint NOT NULL,
    event_type character varying(50) NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT auction_events_sequence_number_check CHECK ((sequence_number > 0))
);


--
-- Name: auctions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.auctions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    status character varying(20) DEFAULT 'WAITING'::character varying NOT NULL,
    current_player_id uuid,
    current_bid numeric(6,2),
    current_bidder_team_id uuid,
    current_round_number integer DEFAULT 1 NOT NULL,
    round_type character varying(10) DEFAULT 'MAIN'::character varying NOT NULL,
    timer_started_at timestamp with time zone,
    timer_deadline_at timestamp with time zone,
    paused_at timestamp with time zone,
    last_controlled_by uuid,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT auctions_current_bid_check CHECK (((current_bid IS NULL) OR (current_bid > (0)::numeric))),
    CONSTRAINT auctions_current_round_number_check CHECK ((current_round_number >= 1)),
    CONSTRAINT auctions_round_type_check CHECK (((round_type)::text = ANY ((ARRAY['MAIN'::character varying, 'UNSOLD'::character varying])::text[]))),
    CONSTRAINT auctions_status_check CHECK (((status)::text = ANY ((ARRAY['WAITING'::character varying, 'ACTIVE'::character varying, 'PAUSED'::character varying, 'RECOVERING'::character varying, 'COMPLETED'::character varying])::text[])))
);


--
-- Name: bids; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bids (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    auction_id uuid NOT NULL,
    room_player_id uuid NOT NULL,
    team_id uuid NOT NULL,
    bid_amount numeric(6,2) NOT NULL,
    bid_sequence bigint NOT NULL,
    request_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT bids_bid_amount_check CHECK ((bid_amount > (0)::numeric)),
    CONSTRAINT bids_bid_sequence_check CHECK ((bid_sequence > 0))
);


--
-- Name: outbox_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outbox_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    auction_id uuid NOT NULL,
    event_id uuid NOT NULL,
    event_type character varying(50) NOT NULL,
    payload jsonb NOT NULL,
    status character varying(20) DEFAULT 'PENDING'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    CONSTRAINT outbox_events_status_check CHECK (((status)::text = ANY ((ARRAY['PENDING'::character varying, 'PROCESSING'::character varying, 'PUBLISHED'::character varying])::text[])))
);


--
-- Name: players; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.players (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    full_name character varying(150) NOT NULL,
    country character varying(100) NOT NULL,
    role character varying(30) NOT NULL,
    is_overseas boolean DEFAULT false NOT NULL,
    base_price numeric(6,2) NOT NULL,
    profile_image_url text,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT players_base_price_check CHECK ((base_price > (0)::numeric)),
    CONSTRAINT players_role_check CHECK (((role)::text = ANY ((ARRAY['BATTER'::character varying, 'BOWLER'::character varying, 'ALL_ROUNDER'::character varying, 'WICKET_KEEPER'::character varying])::text[])))
);


--
-- Name: room_participants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.room_participants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    user_id uuid NOT NULL,
    joined_at timestamp with time zone DEFAULT now() NOT NULL,
    connection_status character varying(20) DEFAULT 'DISCONNECTED'::character varying NOT NULL,
    CONSTRAINT room_participants_connection_status_check CHECK (((connection_status)::text = ANY ((ARRAY['CONNECTED'::character varying, 'DISCONNECTED'::character varying])::text[])))
);


--
-- Name: room_players; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.room_players (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    player_id uuid NOT NULL,
    base_price numeric(6,2) NOT NULL,
    status character varying(20) DEFAULT 'AVAILABLE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT room_players_base_price_check CHECK ((base_price > (0)::numeric)),
    CONSTRAINT room_players_status_check CHECK (((status)::text = ANY ((ARRAY['AVAILABLE'::character varying, 'IN_AUCTION'::character varying, 'SOLD'::character varying, 'UNSOLD'::character varying])::text[])))
);


--
-- Name: rooms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rooms (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_code character varying(12) NOT NULL,
    host_user_id uuid NOT NULL,
    name character varying(100) NOT NULL,
    status character varying(20) DEFAULT 'WAITING'::character varying NOT NULL,
    max_participants smallint DEFAULT 10 NOT NULL,
    starting_purse numeric(6,2) DEFAULT 120.00 NOT NULL,
    min_squad_size smallint DEFAULT 18 NOT NULL,
    max_squad_size smallint DEFAULT 25 NOT NULL,
    max_overseas_players smallint DEFAULT 8 NOT NULL,
    min_bid_increment numeric(4,2) DEFAULT 0.10 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT rooms_check CHECK (((min_squad_size > 0) AND (max_squad_size >= min_squad_size))),
    CONSTRAINT rooms_max_overseas_players_check CHECK ((max_overseas_players >= 0)),
    CONSTRAINT rooms_max_participants_check CHECK (((max_participants >= 4) AND (max_participants <= 10))),
    CONSTRAINT rooms_min_bid_increment_check CHECK ((min_bid_increment > (0)::numeric)),
    CONSTRAINT rooms_starting_purse_check CHECK ((starting_purse > (0)::numeric)),
    CONSTRAINT rooms_status_check CHECK (((status)::text = ANY ((ARRAY['WAITING'::character varying, 'IN_PROGRESS'::character varying, 'COMPLETED'::character varying, 'CANCELLED'::character varying])::text[])))
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone
);


--
-- Name: squads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.squads (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    team_id uuid NOT NULL,
    room_player_id uuid NOT NULL,
    purchase_price numeric(6,2) NOT NULL,
    acquired_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT squads_purchase_price_check CHECK ((purchase_price > (0)::numeric))
);


--
-- Name: teams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.teams (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    participant_id uuid NOT NULL,
    franchise_code character varying(20) NOT NULL,
    franchise_name character varying(100) NOT NULL,
    remaining_purse numeric(6,2) DEFAULT 120.00 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT teams_remaining_purse_check CHECK ((remaining_purse >= (0)::numeric))
);


--
-- Name: unsold_nominations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.unsold_nominations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    auction_id uuid NOT NULL,
    room_player_id uuid NOT NULL,
    team_id uuid NOT NULL,
    round_number integer NOT NULL,
    nominated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT unsold_nominations_round_number_check CHECK ((round_number >= 1))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(255) NOT NULL,
    password_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: auction_events auction_events_auction_id_sequence_number_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auction_events
    ADD CONSTRAINT auction_events_auction_id_sequence_number_key UNIQUE (auction_id, sequence_number);


--
-- Name: auction_events auction_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auction_events
    ADD CONSTRAINT auction_events_pkey PRIMARY KEY (id);


--
-- Name: auctions auctions_id_room_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_id_room_id_key UNIQUE (id, room_id);


--
-- Name: auctions auctions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_pkey PRIMARY KEY (id);


--
-- Name: auctions auctions_room_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_room_id_key UNIQUE (room_id);


--
-- Name: bids bids_auction_id_bid_sequence_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_auction_id_bid_sequence_key UNIQUE (auction_id, bid_sequence);


--
-- Name: bids bids_auction_id_request_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_auction_id_request_id_key UNIQUE (auction_id, request_id);


--
-- Name: bids bids_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_pkey PRIMARY KEY (id);


--
-- Name: outbox_events outbox_events_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_event_id_key UNIQUE (event_id);


--
-- Name: outbox_events outbox_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_pkey PRIMARY KEY (id);


--
-- Name: players players_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.players
    ADD CONSTRAINT players_pkey PRIMARY KEY (id);


--
-- Name: room_participants room_participants_id_room_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_participants
    ADD CONSTRAINT room_participants_id_room_id_key UNIQUE (id, room_id);


--
-- Name: room_participants room_participants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_participants
    ADD CONSTRAINT room_participants_pkey PRIMARY KEY (id);


--
-- Name: room_participants room_participants_room_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_participants
    ADD CONSTRAINT room_participants_room_id_user_id_key UNIQUE (room_id, user_id);


--
-- Name: room_players room_players_id_room_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_players
    ADD CONSTRAINT room_players_id_room_id_key UNIQUE (id, room_id);


--
-- Name: room_players room_players_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_players
    ADD CONSTRAINT room_players_pkey PRIMARY KEY (id);


--
-- Name: room_players room_players_room_id_player_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_players
    ADD CONSTRAINT room_players_room_id_player_id_key UNIQUE (room_id, player_id);


--
-- Name: rooms rooms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rooms
    ADD CONSTRAINT rooms_pkey PRIMARY KEY (id);


--
-- Name: rooms rooms_room_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rooms
    ADD CONSTRAINT rooms_room_code_key UNIQUE (room_code);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_token_hash_key UNIQUE (token_hash);


--
-- Name: squads squads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.squads
    ADD CONSTRAINT squads_pkey PRIMARY KEY (id);


--
-- Name: squads squads_room_player_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.squads
    ADD CONSTRAINT squads_room_player_id_key UNIQUE (room_player_id);


--
-- Name: teams teams_id_room_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_id_room_id_key UNIQUE (id, room_id);


--
-- Name: teams teams_participant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_participant_id_key UNIQUE (participant_id);


--
-- Name: teams teams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_pkey PRIMARY KEY (id);


--
-- Name: teams teams_room_id_franchise_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_room_id_franchise_code_key UNIQUE (room_id, franchise_code);


--
-- Name: unsold_nominations unsold_nominations_auction_id_round_number_team_id_room_pla_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsold_nominations
    ADD CONSTRAINT unsold_nominations_auction_id_round_number_team_id_room_pla_key UNIQUE (auction_id, round_number, team_id, room_player_id);


--
-- Name: unsold_nominations unsold_nominations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsold_nominations
    ADD CONSTRAINT unsold_nominations_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: idx_bids_auction_player_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bids_auction_player_created ON public.bids USING btree (auction_id, room_player_id, created_at);


--
-- Name: idx_outbox_status_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outbox_status_created ON public.outbox_events USING btree (status, created_at);


--
-- Name: idx_room_players_room_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_room_players_room_status ON public.room_players USING btree (room_id, status);


--
-- Name: idx_sessions_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_user_id ON public.sessions USING btree (user_id);


--
-- Name: idx_squads_team_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_squads_team_id ON public.squads USING btree (team_id);


--
-- Name: auction_events auction_events_auction_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auction_events
    ADD CONSTRAINT auction_events_auction_id_room_id_fkey FOREIGN KEY (auction_id, room_id) REFERENCES public.auctions(id, room_id);


--
-- Name: auction_events auction_events_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auction_events
    ADD CONSTRAINT auction_events_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: auctions auctions_current_bidder_team_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_current_bidder_team_id_room_id_fkey FOREIGN KEY (current_bidder_team_id, room_id) REFERENCES public.teams(id, room_id);


--
-- Name: auctions auctions_current_player_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_current_player_id_room_id_fkey FOREIGN KEY (current_player_id, room_id) REFERENCES public.room_players(id, room_id);


--
-- Name: auctions auctions_last_controlled_by_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_last_controlled_by_room_id_fkey FOREIGN KEY (last_controlled_by, room_id) REFERENCES public.room_participants(id, room_id);


--
-- Name: auctions auctions_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auctions
    ADD CONSTRAINT auctions_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: bids bids_auction_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_auction_id_room_id_fkey FOREIGN KEY (auction_id, room_id) REFERENCES public.auctions(id, room_id);


--
-- Name: bids bids_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: bids bids_room_player_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_room_player_id_room_id_fkey FOREIGN KEY (room_player_id, room_id) REFERENCES public.room_players(id, room_id);


--
-- Name: bids bids_team_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bids
    ADD CONSTRAINT bids_team_id_room_id_fkey FOREIGN KEY (team_id, room_id) REFERENCES public.teams(id, room_id);


--
-- Name: outbox_events outbox_events_auction_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_auction_id_room_id_fkey FOREIGN KEY (auction_id, room_id) REFERENCES public.auctions(id, room_id);


--
-- Name: outbox_events outbox_events_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: room_participants room_participants_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_participants
    ADD CONSTRAINT room_participants_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: room_participants room_participants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_participants
    ADD CONSTRAINT room_participants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: room_players room_players_player_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_players
    ADD CONSTRAINT room_players_player_id_fkey FOREIGN KEY (player_id) REFERENCES public.players(id);


--
-- Name: room_players room_players_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.room_players
    ADD CONSTRAINT room_players_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: rooms rooms_host_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rooms
    ADD CONSTRAINT rooms_host_user_id_fkey FOREIGN KEY (host_user_id) REFERENCES public.users(id);


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: squads squads_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.squads
    ADD CONSTRAINT squads_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: squads squads_room_player_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.squads
    ADD CONSTRAINT squads_room_player_id_room_id_fkey FOREIGN KEY (room_player_id, room_id) REFERENCES public.room_players(id, room_id);


--
-- Name: squads squads_team_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.squads
    ADD CONSTRAINT squads_team_id_room_id_fkey FOREIGN KEY (team_id, room_id) REFERENCES public.teams(id, room_id);


--
-- Name: teams teams_participant_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_participant_id_room_id_fkey FOREIGN KEY (participant_id, room_id) REFERENCES public.room_participants(id, room_id);


--
-- Name: teams teams_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: unsold_nominations unsold_nominations_auction_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsold_nominations
    ADD CONSTRAINT unsold_nominations_auction_id_room_id_fkey FOREIGN KEY (auction_id, room_id) REFERENCES public.auctions(id, room_id);


--
-- Name: unsold_nominations unsold_nominations_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsold_nominations
    ADD CONSTRAINT unsold_nominations_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.rooms(id);


--
-- Name: unsold_nominations unsold_nominations_room_player_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsold_nominations
    ADD CONSTRAINT unsold_nominations_room_player_id_room_id_fkey FOREIGN KEY (room_player_id, room_id) REFERENCES public.room_players(id, room_id);


--
-- Name: unsold_nominations unsold_nominations_team_id_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsold_nominations
    ADD CONSTRAINT unsold_nominations_team_id_room_id_fkey FOREIGN KEY (team_id, room_id) REFERENCES public.teams(id, room_id);


--
-- PostgreSQL database dump complete
--


