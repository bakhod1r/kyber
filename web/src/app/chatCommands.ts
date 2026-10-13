// Slash commands in chat (S72), composed by the app shell: Chat parses them, the Issue and Project contexts
// run them with the user's own permissions. The answer is shown only to the one who typed it.
import type { SlashCommand } from "../contexts/chat/domain/model";
import { issueUseCases } from "../contexts/issue/application";
import { projectUseCases } from "../contexts/project/application";
import { identityUseCases } from "../contexts/identity/application";

const projectOf = (issueKey: string) => issueKey.slice(0, issueKey.lastIndexOf("-"));

export async function runChatCommand(cmd: SlashCommand): Promise<string> {
  switch (cmd.name) {
    case "issue": {
      const issue = await issueUseCases.createIssue(cmd.project, cmd.title, "task");
      return `✅ Created ${issue.key}: ${issue.title}`;
    }
    case "assign": {
      const issue = await issueUseCases.issue(cmd.issue);
      let to: { id: string; name: string } | undefined;
      if (cmd.to === "me") {
        const me = await identityUseCases.me();
        to = { id: me.id, name: "you" };
      } else {
        const m = (await projectUseCases.members(projectOf(cmd.issue))).find((x) => x.email.toLowerCase() === cmd.to);
        if (m) to = { id: m.user_id, name: m.name };
      }
      if (!to) throw new Error(`${cmd.to} is not a member of ${projectOf(cmd.issue)}`);
      await issueUseCases.editIssue(issue.key, { version: issue.version, assignee_id: to.id });
      return `👤 ${issue.key} is assigned to ${to.name}`;
    }
    case "status": {
      const wf = await issueUseCases.workflow(projectOf(cmd.issue));
      const want = cmd.status.toLowerCase();
      const st = wf.statuses.find((s) => s.name.toLowerCase() === want || s.id.toLowerCase() === want);
      if (!st) throw new Error(`No status "${cmd.status}" — try: ${wf.statuses.map((s) => s.name).join(", ")}`);
      const issue = await issueUseCases.transition(cmd.issue, st.id);
      return `🔀 ${issue.key} is now ${st.name}`;
    }
  }
}
