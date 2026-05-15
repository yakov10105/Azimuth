namespace Azimuth.Services
{
    public partial class OrderService
    {
        public string OrderId { get; set; }

        public void ProcessOrder(int id)
        {
        }

        public partial void HandleEvent(string eventName);
    }
}
