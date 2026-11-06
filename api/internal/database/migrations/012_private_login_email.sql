UPDATE profile_contacts SET show_email=false WHERE show_email;
ALTER TABLE profile_contacts ADD CONSTRAINT profile_contacts_login_email_private CHECK (NOT show_email);
