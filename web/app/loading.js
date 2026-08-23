import EnterpriseLoader from "../components/EnterpriseLoader";

export default function Loading() {
  return (
    <EnterpriseLoader
      embedded
      message="Loading operational workspace"
      detail="Preparing the latest view and interface modules"
    />
  );
}
