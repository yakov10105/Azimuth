namespace Azimuth.Models
{
    public record Person(string Name, int Age)
    {
        public string DisplayName { get; }

        public bool IsAdult()
        {
            return Age >= 18;
        }

        public string Greet(string greeting)
        {
            return $"{greeting}, {Name}";
        }
    }
}
